package main

import "encoding/json"

func init() {
	examples := map[string]string{
		"butterfly_chart": `{"categories":["North","South","West"],"left":{"name":"Before","values":[20,40,30]},"right":{"name":"After","values":[30,45,40]}}`,
		"mekko_chart":     `{"segments":[{"name":"Enterprise","width":60,"parts":[{"name":"Product","value":60},{"name":"Service","value":40}]},{"name":"SMB","width":40,"parts":[{"name":"Product","value":40},{"name":"Service","value":60}]}]}`,
		"combo_chart":     `{"categories":["Q1","Q2","Q3","Q4"],"bars":[{"name":"Revenue","values":[20,40,30,45]}],"lines":[{"name":"Margin","values":[10,20,15,22]}],"secondary_axis":true,"bar_unit":"$M","line_unit":"%"}`,
	}
	for name, raw := range examples {
		var data map[string]any
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			panic(err)
		}
		visualRecipeData[name] = data
	}
}
