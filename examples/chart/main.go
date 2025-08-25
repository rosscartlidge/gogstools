package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rosscartlidge/gogstools/gs"
)

// ChartConfig defines the configuration for the chart command
type ChartConfig struct {
	X         string              `gs:"field,global,last,help=Use field for X axis"`
	Y         []string            `gs:"field,local,list,help=Use field for Y axis"`
	Match     []map[string]interface{} `gs:"multi,local,list,args=field:content,help=Filter data by field matching content"`
	Right     bool                `gs:"flag,local,last,help=Use right-hand scale"`
	Title     string              `gs:"string,global,last,help=Chart title,default=Chart"`
	Type      string              `gs:"string,global,last,help=Chart type: bar/line/area,default=bar,enum=bar:line:area"`
	Width     float64             `gs:"number,global,last,help=Chart width in pixels,default=800"`
	Height    float64             `gs:"number,global,last,help=Chart height in pixels,default=400"`
	Argv      string              `gs:"file,global,last,help=Input TSV file,suffix=.[tc]sv"`
	Config    string              `gs:"file,global,last,help=Configuration file,suffix=.json"`
	Data      string              `gs:"file,global,last,help=Input data file,suffix=.{tsv,csv}"`
}

// Execute implements the Commander interface
func (cfg *ChartConfig) Execute(ctx context.Context, clauses []gs.ClauseSet) error {
	fmt.Printf("Chart Command Executed!\n")
	fmt.Printf("Title: %s, Type: %s, Size: %.0fx%.0f\n", 
		cfg.Title, cfg.Type, cfg.Width, cfg.Height)
	
	if cfg.X != "" {
		fmt.Printf("X-axis: %s\n", cfg.X)
	}
	
	for i, clause := range clauses {
		fmt.Printf("Clause %d (negated: %v):\n", i+1, clause.IsNegated)
		
		if yFields, ok := clause.Fields["Y"]; ok {
			if yList, ok := yFields.([]interface{}); ok {
				fmt.Printf("  Y fields: %v\n", yList)
			} else {
				fmt.Printf("  Y field: %v\n", yFields)
			}
		}
		
		if right, ok := clause.Fields["Right"]; ok {
			fmt.Printf("  Right axis: %v\n", right)
		}
		
		if matches, ok := clause.Fields["Match"]; ok {
			fmt.Printf("  Match conditions: %v\n", matches)
		}
		
		if args, ok := clause.Fields["_args"]; ok {
			fmt.Printf("  Arguments: %v\n", args)
		}
	}
	
	return nil
}

// Validate implements the Commander interface
func (cfg *ChartConfig) Validate() error {
	validTypes := map[string]bool{
		"bar": true, "line": true, "area": true,
	}
	
	if !validTypes[cfg.Type] {
		return fmt.Errorf("invalid chart type: %s", cfg.Type)
	}
	
	return nil
}

func main() {
	config := &ChartConfig{}
	
	cmd, err := gs.NewCommand(config)
	if err != nil {
		log.Fatalf("Failed to create command: %v", err)
	}
	
	// TSV completion is now integrated into GSCommand
	
	// Execute the command
	if err := cmd.Execute(context.Background(), os.Args[1:]); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}