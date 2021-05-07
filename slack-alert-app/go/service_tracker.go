/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"

	"px.dev/pxapi"
	"px.dev/pxapi/proto/vizierpb"
	"px.dev/pxapi/types"
)

// Alerter is the interface to some sort of alerting platform.
type Alerter interface {
	// SendError alerts with an error. Returns an error if the alerter itself experiences one.
	SendError(msg string) error

	// SendInfo alerts with an info. Returns an error if the alerter itself experiences one.
	SendInfo(msg string) error
}

// ServiceTracker tracks a service.
type ServiceTracker struct {
	alerter   Alerter
	vz        *pxapi.VizierClient
	pxlScript string
}

// NewServiceTracker creates an inits a ServiceTracker.
func NewServiceTracker(alerter Alerter, vz *pxapi.VizierClient) (*ServiceTracker, error) {
	// This PxL script ouputs a table of the HTTP total requests count and
	// HTTP error (>4xxx) count for each service in the `px-sock-shop` namespace.
	// To deploy the px-sock-shop demo, see:
	// https://docs.pixielabs.ai/tutorials/slackbot-alert for how to
	b, err := ioutil.ReadFile("http_errors.pxl")
	if err != nil {
		panic(err)
	}
	pxlScript := string(b)
	return &ServiceTracker{
		alerter:   alerter,
		vz:        vz,
		pxlScript: pxlScript,
	}, nil

}

// Check will run the inner loop of the checker.
func (st *ServiceTracker) Check(ctx context.Context) error {
	tm := &tableMux{tableName: "service_stats"}
	log.Println("Executing PxL script.")
	resultSet, err := st.vz.ExecuteScript(ctx, st.pxlScript, tm)
	if err != nil {
		return fmt.Errorf("Got error: %+v, on execute script", err)
	}

	log.Println("Stream PxL script results.")
	if err := resultSet.Stream(); err != nil {
		return fmt.Errorf("Got error: %+v, while streaming", err)
	}

	// Get slack message constructed from table data.
	table := tm.GetTable()
	if table == nil {
		return fmt.Errorf("Unable to find expected table '%s'", tm.tableName)
	}

	if !table.HasIncidents() {
		log.Println("Not sending alerts as there are no active incidents")
		return nil
	}

	log.Println("Sending alert.")
	err = st.alerter.SendInfo(table.SummarizeIncidents())
	if err != nil {
		return fmt.Errorf("Got error: %+v, while streaming", err.Error())
	}
	return nil
}

// Implement the TableRecordHandler interface to processes the PxL script output table record-wise.
type tableCollector struct {
	mgr IncidentManager
	// Channel used to block until all of the table data to be collected.
	done chan struct{}
}

func (t *tableCollector) HandleInit(ctx context.Context, metadata types.TableMetadata) error {
	return nil
}

func isFloat(d types.Datum) bool {
	return d != nil && d.Type() == vizierpb.FLOAT64
}

func toFloat(d types.Datum) float64 {
	if !isFloat(d) {
		panic(fmt.Errorf("Cannot convert %s to float", d.Type()))
	}
	return d.(*types.Float64Value).Value()

}

// IncidentData the data bout an incident.
type IncidentData struct {
	Service                string
	MaxError               float64
	PercentExceedThreshold float64
}

// IncidentManager handles any incidents that occur
type IncidentManager interface {
	// Add an incident.
	UpsertIncident(service string, data *IncidentData)
	// Summarize the incidents.
	Summarize() string

	// Returns the number of active incidents
	NumActiveIncidents() int
}

// Manages an incident without any memory between queries
type singleQueryIncidentManager struct {
	data map[string]*IncidentData
}

func (s *singleQueryIncidentManager) UpsertIncident(service string, data *IncidentData) {
	s.data[service] = data
}

func (s *singleQueryIncidentManager) NumActiveIncidents() int {
	return len(s.data)
}

func (s *singleQueryIncidentManager) Summarize() string {
	threshold := 5.0
	lines := make([]string, 0)
	for service, incident := range s.data {
		lines = append(lines, fmt.Sprintf("`%s` \t ---> `%4.1f%%`  windows exceed %.3g%% error threshold. Max error: `%4.1f %%`",
			service, incident.PercentExceedThreshold*100, threshold, incident.MaxError*100))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (t *tableCollector) addIncident(service string, data *types.Record) error {
	percentExceedThreshold := data.GetDatum("percent_exceed_threshold")
	maxError := data.GetDatum("max_error")

	if !isFloat(maxError) || !isFloat(percentExceedThreshold) {
		return fmt.Errorf("error parsing data")
	}
	t.mgr.UpsertIncident(service, &IncidentData{
		Service:                service,
		MaxError:               toFloat(maxError),
		PercentExceedThreshold: toFloat(percentExceedThreshold),
	})
	return nil
}

func (t *tableCollector) HandleRecord(ctx context.Context, r *types.Record) error {
	percentExceedThreshold := r.GetDatum("percent_exceed_threshold")
	if toFloat(percentExceedThreshold) > 0 {
		service := r.GetDatum("service")
		t.addIncident(service.String(), r)
	}

	return nil
}

func (t *tableCollector) HandleDone(ctx context.Context) error {
	close(t.done)
	return nil
}

func (t *tableCollector) HasIncidents() bool {
	return t.mgr.NumActiveIncidents() > 0
}

func (t *tableCollector) SummarizeIncidents() string {
	// Wait until the `done` channel is closed, indicating table data has finished collecting.
	if t == nil {
		panic(fmt.Errorf("Table not found"))
	}
	<-t.done
	return t.mgr.Summarize()
}

// Implement the TableMuxer to route pxl script output tables to the correct handler.
type tableMux struct {
	tableName string
	table     *tableCollector
}

func (s *tableMux) AcceptTable(ctx context.Context, metadata types.TableMetadata) (pxapi.TableRecordHandler, error) {
	if metadata.Name != s.tableName {
		return nil, nil
	}
	s.table = &tableCollector{done: make(chan struct{}), mgr: &singleQueryIncidentManager{data: make(map[string]*IncidentData)}}
	return s.table, nil
}

func (s *tableMux) GetTable() *tableCollector {
	return s.table
}
