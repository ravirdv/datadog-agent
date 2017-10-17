// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2017 Datadog, Inc.

// +build docker

package ecs

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/DataDog/datadog-agent/pkg/config"
    dockerutil "github.com/DataDog/datadog-agent/pkg/util/docker"
    "github.com/docker/docker/client"
)

const (
    // DefaultAgentPort is the default port used by the ECS Agent.
    DefaultAgentPort = 51678
    // DefaultECSContainer is the default container used by ECS.
    DefaultECSContainer = "amazon-ecs-agent"
)

// DetectedAgentURL stores the URL of the ECS agent. After the first call to
// getHostname this will be detected and used as-is going forward. It will only
// be re-detected if getHostname is called again.
var detectedAgentURL string

type (
    // CommandsV1Response is the format of a response from the ECS-agent on the root.
    CommandsV1Response struct {
        AvailableCommands []string `json:"AvailableCommands"`
    }

    // TasksV1Response is the format of a response from the ECS tasks API.
    // See http://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-agent-introspection.html
    TasksV1Response struct {
        Tasks []TaskV1 `json:"tasks"`
    }

    // TaskV1 is the format of a Task in the ECS tasks API.
    TaskV1 struct {
        Arn           string        `json:"Arn"`
        DesiredStatus string        `json:"DesiredStatus"`
        KnownStatus   string        `json:"KnownStatus"`
        Family        string        `json:"Family"`
        Version       string        `json:"Version"`
        Containers    []ContainerV1 `json:"containers"`
    }

    // ContainerV1 is the format of a Container in the ECS tasks API.
    ContainerV1 struct {
        DockerID   string `json:"DockerId"`
        DockerName string `json:"DockerName"`
        Name       string `json:"Name"`
    }
)

// IsInstance returns whether this host is part of an ECS cluster
func IsInstance() bool {
    if detectedAgentURL == "" {
        _, err := detectAgentURL()
        if err != nil {
            return false
        }
        return true
    }
    return true
}

// IsAgentNotDetected indicates if an error from ExtractPayload was about no
// ECS agent being detected. This is a used as a way to check if is available
// or if the host is not in an ECS cluster.
func IsAgentNotDetected(err error) bool {
    return strings.Contains(err.Error(), "could not detect ECS agent")
}

// ExtractPayload returns a TasksV1Response containing information about the state
// of the local ECS containers running on this node. This data is provided via
// the local ECS agent.
func ExtractPayload() (TasksV1Response, error) {

    var resp TasksV1Response
    if detectedAgentURL == "" {
        url, err := detectAgentURL()
        if err != nil {
            return resp, err
        }
        detectedAgentURL = url
    }
    // TODO: Use IsAgentNotDetected ?

    r, err := http.Get(fmt.Sprintf("%sv1/tasks", detectedAgentURL))
    if err != nil {
        return resp, err
    }
    defer r.Body.Close()

    if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
        return resp, err
    }
    return resp, nil
}

// detectAgentURL finds a hostname for the ECS-agent either via Docker, if
// running inside of a container, or just defaulting to localhost.
func detectAgentURL() (string, error) {
    urls := make([]string, 0, 3)
    if config.IsContainerized() {
        cli, err := dockerutil.ConnectToDocker()
        if err != nil {
            return "", err
        }
        defer cli.Close()

        // Try all networks available on the ecs container.
        ecsConfig, err := cli.ContainerInspect(context.TODO(), DefaultECSContainer)
        if client.IsErrContainerNotFound(err) {
            return "", fmt.Errorf("could not detect ECS agent, missing %s container", DefaultECSContainer)
        } else if err != nil {
            return "", err
        }
        for _, network := range ecsConfig.NetworkSettings.Networks {
            ip := network.IPAddress
            if ip != "" {
                urls = append(urls, fmt.Sprintf("http://%s:%d/", ip, DefaultAgentPort))
            }
        }

        // Try the default gateway
        gw, err := dockerutil.DefaultGateway()
        if err != nil {
            // "expected" errors are handled in DefaultGateway so only
            // unexpected errors are bubbled up, so we keep bubbling.
            return "", err
        }
        if gw != nil {
            urls = append(urls, fmt.Sprintf("http://%s:%d/", gw.String(), DefaultAgentPort))
        }
    }

    // Always try the localhost URL.
    urls = append(urls, fmt.Sprintf("http://localhost:%d/", DefaultAgentPort))
    detected := testURLs(urls, 1*time.Second)
    if detected != "" {
        return detected, nil
    }
    return "", fmt.Errorf("could not detect ECS agent, tried URLs: %s", urls)
}

// testURLs trys a set of URLs and returns the first one that succeeds.
func testURLs(urls []string, timeout time.Duration) string {
    client := &http.Client{Timeout: timeout}
    for _, url := range urls {
        r, err := client.Get(url)
        if err != nil {
            continue
        }
        if r.StatusCode != http.StatusOK {
            continue
        }
        var resp CommandsV1Response
        if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
            fmt.Printf("decode err: %s\n", err)
            continue
        }
        if len(resp.AvailableCommands) > 0 {
            return url
        }
    }
    return ""
}
