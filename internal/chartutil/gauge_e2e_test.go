package chartutil

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/safeyaml"
)

func TestGaugeE2E_DataFlow(t *testing.T) {
	// Simulate the exact YAML used in test fixtures
	yamlContent := `type: gauge
title: Customer Satisfaction
data:
  - label: Score
    value: 85
`
	
	var chartData map[string]interface{}
	if err := safeyaml.UnmarshalString(yamlContent, &chartData); err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}
	
	t.Logf("chartData: %+v", chartData)
	t.Logf("chartData['data'] type: %T", chartData["data"])
	
	// Check the data field
	dataField := chartData["data"]
	switch d := dataField.(type) {
	case []interface{}:
		t.Logf("data is []interface{} with %d items", len(d))
		for i, item := range d {
			t.Logf("  item[%d]: %+v (type: %T)", i, item, item)
			if m, ok := item.(map[string]interface{}); ok {
				for k, v := range m {
					t.Logf("    %s = %v (type: %T)", k, v, v)
				}
			}
		}
	case map[string]interface{}:
		t.Logf("data is map with %d keys", len(d))
	default:
		t.Logf("data is unexpected type: %T", dataField)
	}
	
	// Build the payload
	payload := BuildChartDataPayload(chartData, yamlContent)
	t.Logf("Payload: %+v", payload)
	
	// Verify the value
	value, ok := payload["value"]
	if !ok {
		t.Fatal("Payload missing 'value' key!")
	}
	t.Logf("value = %v (type: %T)", value, value)
	
	if vf, ok := value.(float64); !ok {
		t.Errorf("value is not float64, got %T", value)
	} else if vf != 85.0 {
		t.Errorf("Expected value 85.0, got %v", vf)
	}
	
	minVal := payload["min"]
	maxVal := payload["max"]
	t.Logf("min = %v (type: %T)", minVal, minVal)
	t.Logf("max = %v (type: %T)", maxVal, maxVal)
}

func TestGaugeE2E_MapFormat(t *testing.T) {
	// Test the less-common map format
	yamlContent := `type: gauge
title: Health
data:
  Score: 92
`
	
	var chartData map[string]interface{}
	if err := safeyaml.UnmarshalString(yamlContent, &chartData); err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}
	
	t.Logf("chartData['data'] type: %T, value: %+v", chartData["data"], chartData["data"])
	
	payload := BuildChartDataPayload(chartData, yamlContent)
	t.Logf("Payload: %+v", payload)
	
	value := payload["value"]
	t.Logf("value = %v (type: %T)", value, value)
	
	if vf, ok := value.(float64); !ok {
		t.Errorf("value is not float64, got %T", value)
	} else if vf != 92.0 {
		t.Errorf("Expected value 92.0, got %v", vf)
	}
}
