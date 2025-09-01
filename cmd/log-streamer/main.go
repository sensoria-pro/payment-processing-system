package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

const (
	composeFileName = "docker-compose.yml"
	projectRootRelPath = "../.."
) 

var colorPalette = []*color.Color{
	color.New(color.FgCyan),
	color.New(color.FgGreen),
	color.New(color.FgYellow),
	color.New(color.FgBlue),
	color.New(color.FgMagenta),
	color.New(color.FgRed),
}

// Structure for parsing docker-compose.yml
type ComposeConfig struct {
	Services map[string]interface{} `yaml:"services"`
}

func main() {
	// 1. Setting up context for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down log streamer...")
		cancel()
	}()

	// 2. Initializing the Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("❌ Failed to create Docker client: %v", err)
	}
	defer func() {
		if err := cli.Close(); err != nil {
			log.Printf("⚠️  Error closing Docker client: %v", err)
		}
	}()

	// 3. Parsing docker-compose.yml to get service names
	composePath := filepath.Join(projectRootRelPath, composeFileName)
	composeFile, err := os.ReadFile(composePath)
	if err != nil {
		log.Fatalf("❌ Failed to read %s: %v", composePath, err)
	}

	var config ComposeConfig
	if err := yaml.Unmarshal(composeFile, &config); err != nil {
		log.Fatalf("❌ Failed to parse docker-compose.yaml: %v", err)
	}

	// 4. Run log streaming for each service in a separate goroutine
	var wg sync.WaitGroup
	i := 0
	log.Println("Starting log streams...")

	for serviceName := range config.Services {
		wg.Add(1)
		// Color to a service cyclically from a palette
		serviceColor := colorPalette[i%len(colorPalette)]
		go streamServiceLogs(ctx, &wg, cli, serviceName, serviceColor)
		i++
	}

	wg.Wait()
	log.Println("All log streams finished.")
}

func streamServiceLogs(ctx context.Context, wg *sync.WaitGroup, cli *client.Client, serviceName string, c *color.Color) {
	defer wg.Done()

	// Find a container by service name
	// In docker-compose, a container name usually looks like: <project_name>-<service_name>-1
	// We will look for a container that has the label "com.docker.compose.service"
	containers, err := cli.ContainerList(ctx, containerTypes.ListOptions{})
	if err != nil {
		log.Printf("⚠️  Error listing containers for %s: %v", serviceName, err)
		return
	}

	var containerID string
	for _, cont := range containers {
		if cont.Labels["com.docker.compose.service"] == serviceName {
			containerID = cont.ID
			break
		}
	}

	if containerID == "" {
		log.Printf("⚠️  Container for service %s not found.", serviceName)
		return
	}

	// Let's start streaming logs
	logReader, err := cli.ContainerLogs(ctx, containerID, containerTypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		log.Printf("⚠️  Error getting logs for %s: %v", serviceName, err)
		return
	}
	defer func() {
		if err := logReader.Close(); err != nil {
			log.Printf("⚠️  Error closing log reader for %s: %v", serviceName, err)
		}
	}()

	// We read the log stream and output it to the console with color
	scanner := bufio.NewScanner(logReader)
	for scanner.Scan() {
		prefix := c.SprintfFunc()("[%s]", serviceName)
		fmt.Printf("%-25s %s\n", prefix, scanner.Text())
	}
}
