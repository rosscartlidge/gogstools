package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"html/template"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/rosscartlidge/gogstools/gs"
)

// ChartConfig defines the configuration for the chart command
type ChartConfig struct {
	X      string                      `gs:"field,global,last,help=Use field for X axis"`
	Y      []string                    `gs:"field,local,list,help=Use field for Y axis"`
	Match  []map[string]interface{}    `gs:"multi,local,list,args=field:content,help=Filter data by field matching content"`
	Right  bool                        `gs:"flag,local,last,help=Use right-hand scale"`
	Title  string                      `gs:"string,global,last,help=Chart title,default=Chart"`
	Type   string                      `gs:"string,global,last,help=Chart type: bar/line/area,default=bar,enum=bar:line:area"`
	Width  float64                     `gs:"number,global,last,help=Chart width in pixels,default=800"`
	Height float64                     `gs:"number,global,last,help=Chart height in pixels,default=400"`
	Quiet  bool                        `gs:"flag,global,last,help=Suppress progress messages,default=true"`
	Argv   string                      `gs:"file,global,last,help=Input TSV file,suffix=.[tc]sv"`
}

// Dataset represents a Chart.js dataset
type Dataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BackgroundColor string    `json:"backgroundColor"`
	BorderColor     string    `json:"borderColor"`
	YAxisID         string    `json:"yAxisID,omitempty"`
	Fill            bool      `json:"fill"`
}

// ChartData represents the complete chart data structure
type ChartData struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

// ChartOptions represents Chart.js configuration options
type ChartOptions struct {
	Responsive bool              `json:"responsive"`
	Scales     map[string]Scale  `json:"scales"`
	Plugins    map[string]Plugin `json:"plugins"`
}

type Scale struct {
	Type     string `json:"type"`
	Position string `json:"position,omitempty"`
	Display  bool   `json:"display"`
	Title    Title  `json:"title"`
}

type Title struct {
	Display bool   `json:"display"`
	Text    string `json:"text"`
}

type Plugin struct {
	Display bool   `json:"display,omitempty"`
	Text    string `json:"text,omitempty"`
}

// TSVData represents parsed TSV data
type TSVData struct {
	Headers []string
	Rows    [][]string
}

// getInputFile determines the input file from args or Argv field, returns "-" for stdin
func (cfg *ChartConfig) getInputFile(clauses []gs.ClauseSet) string {
	// First check if Argv is set (either from -argv flag or bare argument)
	if cfg.Argv != "" {
		return cfg.Argv
	}
	
	// Check for _args in any clause (bare arguments)
	for _, clause := range clauses {
		if args, ok := clause.Fields["_args"]; ok {
			if argList, ok := args.([]string); ok && len(argList) > 0 {
				for _, arg := range argList {
					if strings.HasSuffix(strings.ToLower(arg), ".tsv") || 
					   strings.HasSuffix(strings.ToLower(arg), ".csv") {
						return arg
					}
				}
			}
		}
	}
	
	// If no file specified, use stdin (for pipe support)
	return "-"
}

// parseTSV reads and parses a TSV/CSV file or stdin
func parseTSV(filename string) (*TSVData, error) {
	var reader *bufio.Scanner
	var closeFn func() error
	
	// Handle stdin vs file input
	if filename == "" || filename == "-" {
		reader = bufio.NewScanner(os.Stdin)
		closeFn = func() error { return nil } // Don't close stdin
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("opening file %s: %w", filename, err)
		}
		reader = bufio.NewScanner(file)
		closeFn = file.Close
	}
	defer closeFn()
	
	// Read first line to detect separator
	if !reader.Scan() {
		if err := reader.Err(); err != nil {
			return nil, fmt.Errorf("reading first line: %w", err)
		}
		return nil, fmt.Errorf("no data in input")
	}
	
	firstLine := reader.Text()
	
	// Find separator: first non-variable character, default to tab
	separator := '\t'
	for _, r := range firstLine {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			separator = r
			break
		}
	}
	
	// Parse first line as headers
	headers := strings.Split(firstLine, string(separator))
	for i, header := range headers {
		headers[i] = strings.TrimSpace(header)
	}
	
	// Parse remaining lines
	var rows [][]string
	for reader.Scan() {
		line := reader.Text()
		if line == "" {
			continue // Skip empty lines
		}
		fields := strings.Split(line, string(separator))
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}
		rows = append(rows, fields)
	}
	
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}
	
	return &TSVData{
		Headers: headers,
		Rows:    rows,
	}, nil
}

// findFieldIndex returns the index of a field in the headers
func (data *TSVData) findFieldIndex(fieldName string) int {
	for i, header := range data.Headers {
		if header == fieldName {
			return i
		}
	}
	return -1
}

// filterData applies match conditions to filter the TSV data
func (data *TSVData) filterData(matches []map[string]interface{}) *TSVData {
	if len(matches) == 0 {
		return data
	}
	
	filteredRows := [][]string{}
	
	for _, row := range data.Rows {
		include := true
		
		// All match conditions in the same clause must be satisfied (AND logic)
		for _, match := range matches {
			fieldName, hasField := match["field"].(string)
			content, hasContent := match["content"].(string)
			
			if !hasField || !hasContent {
				continue
			}
			
			fieldIndex := data.findFieldIndex(fieldName)
			if fieldIndex == -1 || fieldIndex >= len(row) {
				include = false
				break
			}
			
			// Simple regex matching
			matched, err := regexp.MatchString(content, row[fieldIndex])
			if err != nil || !matched {
				include = false
				break
			}
		}
		
		if include {
			filteredRows = append(filteredRows, row)
		}
	}
	
	return &TSVData{
		Headers: data.Headers,
		Rows:    filteredRows,
	}
}

// generateColor creates a deterministic color from field name using MD5
func generateColor(fieldName string) (string, string) {
	hash := md5.Sum([]byte(fieldName))
	
	// Convert hash to RGB values
	r := int(hash[0])
	g := int(hash[1])  
	b := int(hash[2])
	
	// Create background (with alpha) and border colors
	bgColor := fmt.Sprintf("rgba(%d, %d, %d, 0.6)", r, g, b)
	borderColor := fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
	
	return bgColor, borderColor
}

// Execute implements the Commander interface
func (cfg *ChartConfig) Execute(ctx context.Context, clauses []gs.ClauseSet) error {
	// Get input file
	inputFile := cfg.getInputFile(clauses)
	if inputFile == "" {
		return fmt.Errorf("no input file specified")
	}
	
	// Parse TSV data
	data, err := parseTSV(inputFile)
	if err != nil {
		return fmt.Errorf("parsing TSV file: %w", err)
	}
	
	if cfg.X == "" {
		return fmt.Errorf("X axis field must be specified with -x")
	}
	
	xIndex := data.findFieldIndex(cfg.X)
	if xIndex == -1 {
		return fmt.Errorf("X field '%s' not found in data", cfg.X)
	}
	
	// Process each clause to create datasets
	chartData := ChartData{Labels: []string{}, Datasets: []Dataset{}}
	
	// Extract labels from X field
	for _, row := range data.Rows {
		if xIndex < len(row) {
			chartData.Labels = append(chartData.Labels, row[xIndex])
		}
	}
	
	for i, clause := range clauses {
		// Apply filtering if match conditions exist
		filteredData := data
		if matches, ok := clause.Fields["Match"]; ok {
			if matchList, ok := matches.([]interface{}); ok {
				matchConditions := []map[string]interface{}{}
				for _, m := range matchList {
					if matchMap, ok := m.(map[string]interface{}); ok {
						matchConditions = append(matchConditions, matchMap)
					}
				}
				filteredData = data.filterData(matchConditions)
			}
		}
		
		// Determine if this clause uses right axis
		useRightAxis := false
		if right, ok := clause.Fields["Right"]; ok {
			if rightBool, ok := right.(bool); ok {
				useRightAxis = rightBool
			}
		}
		
		// Process Y fields for this clause
		if yFields, ok := clause.Fields["Y"]; ok {
			var yFieldNames []string
			
			// Handle both single fields and lists
			if yList, ok := yFields.([]interface{}); ok {
				for _, field := range yList {
					if fieldStr, ok := field.(string); ok {
						yFieldNames = append(yFieldNames, fieldStr)
					}
				}
			} else if yField, ok := yFields.(string); ok {
				yFieldNames = []string{yField}
			}
			
			// Create dataset for each Y field
			for _, yField := range yFieldNames {
				yIndex := filteredData.findFieldIndex(yField)
				if yIndex == -1 {
					log.Printf("Warning: Y field '%s' not found in data", yField)
					continue
				}
				
				// Extract numeric data
				yData := []float64{}
				for _, row := range filteredData.Rows {
					if yIndex < len(row) {
						if val, err := strconv.ParseFloat(row[yIndex], 64); err == nil {
							yData = append(yData, val)
						} else {
							yData = append(yData, 0) // Default to 0 for non-numeric values
						}
					}
				}
				
				// Generate deterministic colors
				bgColor, borderColor := generateColor(yField)
				
				// Create dataset
				dataset := Dataset{
					Label:           yField,
					Data:            yData,
					BackgroundColor: bgColor,
					BorderColor:     borderColor,
					Fill:            cfg.Type == "area",
				}
				
				if useRightAxis {
					dataset.YAxisID = "y1"
				} else {
					dataset.YAxisID = "y"
				}
				
				chartData.Datasets = append(chartData.Datasets, dataset)
			}
		}
		
		// Log clause processing if verbose mode is enabled
		if !cfg.Quiet {
			log.Printf("Processed clause %d: %d Y fields, right axis: %v", 
				i+1, len(chartData.Datasets), useRightAxis)
		}
	}
	
	// Generate Chart.js configuration
	err = cfg.generateChart(chartData)
	if err != nil {
		return fmt.Errorf("generating chart: %w", err)
	}
	
	return nil
}

// generateChart outputs the HTML with Chart.js
func (cfg *ChartConfig) generateChart(data ChartData) error {
	// Determine Chart.js chart type
	chartType := cfg.Type
	if chartType == "area" {
		chartType = "line" // Chart.js uses line charts with fill for area charts
	}
	
	// Check if we need dual axes
	needsRightAxis := false
	for _, dataset := range data.Datasets {
		if dataset.YAxisID == "y1" {
			needsRightAxis = true
			break
		}
	}
	
	// Build scales configuration
	scales := map[string]Scale{
		"x": {
			Display: true,
			Title:   Title{Display: true, Text: cfg.X},
		},
		"y": {
			Type:     "linear",
			Display:  true,
			Position: "left",
			Title:    Title{Display: true, Text: "Values"},
		},
	}
	
	if needsRightAxis {
		scales["y1"] = Scale{
			Type:     "linear",
			Display:  true,
			Position: "right",
			Title:    Title{Display: true, Text: "Right Axis"},
		}
	}
	
	options := ChartOptions{
		Responsive: true,
		Scales:     scales,
		Plugins: map[string]Plugin{
			"title": {
				Display: true,
				Text:    cfg.Title,
			},
		},
	}
	
	// HTML template
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        #chartContainer { width: {{.Width}}px; height: {{.Height}}px; margin: 0 auto; }
    </style>
</head>
<body>
    <div id="chartContainer">
        <canvas id="myChart"></canvas>
    </div>
    
    <script>
        const ctx = document.getElementById('myChart').getContext('2d');
        const chart = new Chart(ctx, {
            type: '{{.ChartType}}',
            data: {{.DataJSON}},
            options: {{.OptionsJSON}}
        });
    </script>
</body>
</html>`
	
	// Convert data to JSON (manually to avoid import)
	dataJSON := fmt.Sprintf(`{
		"labels": [%s],
		"datasets": [%s]
	}`, 
		formatLabels(data.Labels),
		formatDatasets(data.Datasets))
	
	optionsJSON := formatOptions(options)
	
	// Execute template
	t := template.Must(template.New("chart").Parse(tmpl))
	templateData := struct {
		Title       string
		Width       float64
		Height      float64
		ChartType   string
		DataJSON    template.JS
		OptionsJSON template.JS
	}{
		Title:       cfg.Title,
		Width:       cfg.Width,
		Height:      cfg.Height,
		ChartType:   chartType,
		DataJSON:    template.JS(dataJSON),
		OptionsJSON: template.JS(optionsJSON),
	}
	
	return t.Execute(os.Stdout, templateData)
}

// Helper functions for JSON formatting (avoiding external JSON library)
func formatLabels(labels []string) string {
	quoted := make([]string, len(labels))
	for i, label := range labels {
		quoted[i] = fmt.Sprintf("%q", label)
	}
	return strings.Join(quoted, ", ")
}

func formatDatasets(datasets []Dataset) string {
	formatted := make([]string, len(datasets))
	for i, ds := range datasets {
		dataValues := make([]string, len(ds.Data))
		for j, val := range ds.Data {
			dataValues[j] = fmt.Sprintf("%.2f", val)
		}
		
		yAxisPart := ""
		if ds.YAxisID != "" {
			yAxisPart = fmt.Sprintf(`, "yAxisID": "%s"`, ds.YAxisID)
		}
		
		formatted[i] = fmt.Sprintf(`{
			"label": %q,
			"data": [%s],
			"backgroundColor": "%s",
			"borderColor": "%s",
			"fill": %t%s
		}`, ds.Label, strings.Join(dataValues, ", "), 
			ds.BackgroundColor, ds.BorderColor, ds.Fill, yAxisPart)
	}
	return strings.Join(formatted, ", ")
}

func formatOptions(options ChartOptions) string {
	scalesJSON := "{"
	first := true
	for key, scale := range options.Scales {
		if !first {
			scalesJSON += ", "
		}
		scalesJSON += fmt.Sprintf(`"%s": {
			"display": %t,
			"position": "%s",
			"title": {
				"display": %t,
				"text": "%s"
			}
		}`, key, scale.Display, scale.Position, 
			scale.Title.Display, scale.Title.Text)
		first = false
	}
	scalesJSON += "}"
	
	return fmt.Sprintf(`{
		"responsive": %t,
		"scales": %s,
		"plugins": {
			"title": {
				"display": %t,
				"text": "%s"
			}
		}
	}`, options.Responsive, scalesJSON,
		options.Plugins["title"].Display, options.Plugins["title"].Text)
}

// Validate implements the Commander interface
func (cfg *ChartConfig) Validate() error {
	// Enum validation now handled during parsing
	return nil
}

func main() {
	config := &ChartConfig{}
	
	cmd, err := gs.NewCommand(config)
	if err != nil {
		log.Fatalf("Failed to create command: %v", err)
	}
	
	// Execute the command
	if err := cmd.Execute(context.Background(), os.Args[1:]); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}