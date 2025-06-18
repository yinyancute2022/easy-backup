package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"easy-backup/internal/config"
)

func main() {
	var (
		configPath = flag.String("config", "config.yaml", "Path to configuration file")
		validate   = flag.Bool("validate", false, "Validate configuration only")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *validate {
		fmt.Println("Configuration is valid!")
		os.Exit(0)
	}

	// Print configuration summary
	fmt.Printf("Configuration loaded successfully from: %s\n", *configPath)
	fmt.Printf("Log Level: %s\n", cfg.Global.LogLevel)
	fmt.Printf("Timezone: %s\n", cfg.Global.Timezone)
	fmt.Printf("S3 Bucket: %s\n", cfg.Global.S3.Bucket)
	fmt.Printf("Strategies: %d\n", len(cfg.Strategies))

	for _, strategy := range cfg.Strategies {
		fmt.Printf("  - %s (schedule: %s, retention: %s)\n",
			strategy.Name, strategy.Schedule, strategy.Retention)
	}
}
