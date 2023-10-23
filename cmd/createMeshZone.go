// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// createMeshZoneCmd represents the upsertGlobalControlPlane command
var createMeshZoneCmd = &cobra.Command{
	Use:   "create-zone",
	Short: "Creates a zone in the specified global control plane",
// 	Long: `A longer description that spans multiple lines and likely contains examples
// and usage of using your command. For example:

// Cobra is a CLI library for Go that empowers applications.
// This application is a tool to generate the needed files
// to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		createGlobalControlPlaneResponse := createGlobalControlPlane()
		createMeshZoneResponse := createMeshZone(createGlobalControlPlaneResponse)
		CreateSecret(createGlobalControlPlaneResponse.ID+"-"+createMeshZoneResponse.Name, createMeshZoneResponse.Token)
		var output CreateMeshZoneOutput
		output.GlobalControlPlaneName = createGlobalControlPlaneResponse.Name
		output.GlobalControlPlaneId = createGlobalControlPlaneResponse.ID
		output.ZoneName = createMeshZoneResponse.Name
		output.ZoneTokenSecretName = createGlobalControlPlaneResponse.ID+"-"+createMeshZoneResponse.Name
		
		outputJson, _ := json.Marshal(output)
		fmt.Printf("%s\n", outputJson)
		
	},
}

func init() {
	meshManagerCmd.AddCommand(createMeshZoneCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// upsertGlobalControlPlaneCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	createMeshZoneCmd.Flags().StringVar(&kong_api_endpoint, "api-endpoint", "https://us.api.konghq.com", "API endpoint for the control plane")
	createMeshZoneCmd.Flags().StringVar(&mesh_api_version, "api-version", "v0", "Konnect API version")
	createMeshZoneCmd.Flags().StringVar(&personal_access_token, "personal-access-token", "", "Kong Konnect Personal Access Token")
	createMeshZoneCmd.Flags().StringVar(&mesh_control_plane_name, "control-plane-name", "default", "Name for the global control plane")
	createMeshZoneCmd.Flags().StringVar(&mesh_zone_name, "zone-name", "default", "Name for the zone")
}

// type GlobalControlPlane struct {
// 	Name string `json:"name"`
// }

type GlobalControlPlaneList struct {
	Meta struct {
		Page struct {
			Size  int `json:"size"`
			Total int `json:"total"`
		} `json:"page"`
	} `json:"meta"`
	Data [] GlobalControlPlane `json:"data"`
}

type GlobalControlPlane struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Labels      struct {
		Test string `json:"test"`
	} `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MeshZone struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Token       string
}

type CreateMeshZoneResponse struct {
	Token          string `json:"token"`
}

type CreateMeshZoneOutput struct {
	GlobalControlPlaneName                 	string `json:"global_control_plane_name"`
	GlobalControlPlaneId    				string `json:"global_control_plane_id"`
	ZoneName    							string `json:"zone_name"`
	ZoneTokenSecretName  					string `json:"zone_token_secret_name"`
}


// function to call Kong Konnect API and create a global control plane
func createGlobalControlPlane() GlobalControlPlane {

	client := &http.Client{}
	globalControlPlane := GlobalControlPlane{}

	//Make a get request to the Kong Konnect API to find if the global control plane already exists
	globelControlPlaneListReq, err := http.NewRequest(http.MethodGet, kong_api_endpoint+"/"+mesh_api_version+"/mesh/control-planes", nil)
	
	if err != nil {
		log.Fatal("Error creating request:", err)
	}

	globelControlPlaneListReq.Header.Set("Content-Type", "application/json")
	globelControlPlaneListReq.Header.Set("Authorization", "Bearer "+personal_access_token)
	globelControlPlaneListReq.Header.Set("Accept", "application/json")

	
	globelControlPlaneListResponse, err := client.Do(globelControlPlaneListReq)
	if err != nil {
		log.Fatal("Error sending request:", err)
	}
	defer globelControlPlaneListResponse.Body.Close()

	var globalControlPlaneListResponse GlobalControlPlaneList

	err = json.NewDecoder(globelControlPlaneListResponse.Body).Decode(&globalControlPlaneListResponse)

	if err != nil {
		log.Fatal("Error decoding data:", err)
	}

	// check if globalControlPlaneListResponse.Data is not empty
	if len(globalControlPlaneListResponse.Data) > 0 {
		// iterate through globalControlPlaneListResponse.Data[].Name and check if it matches with the name provided by the user
		for _, globalControlPlane := range globalControlPlaneListResponse.Data {
			if strings.EqualFold(globalControlPlane.Name,mesh_control_plane_name) {
				fmt.Println("Detected existing Global Control Plane, skipping creation")
				return globalControlPlane
			}
		}
	}

	reqBody, err := json.Marshal(map[string]string{
		"name": mesh_control_plane_name,
	})

	if err != nil {
		log.Fatal("Error marshaling request body:", err)
	}

	req, err := http.NewRequest(http.MethodPost, kong_api_endpoint+"/"+mesh_api_version+"/mesh/control-planes", bytes.NewBuffer(reqBody))
	
	if err != nil {
		log.Fatal("Error creating request:", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personal_access_token)
	req.Header.Set("Accept", "application/json")


	resp, err := client.Do(req)

	if err != nil {
			log.Fatal("Error sending request:", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		log.Fatal("UnAuthorized, pls validate your PAT token and Konnect RBAC permissions")
	}

	if resp.StatusCode == 201 {
		fmt.Println("Global Control Plane created")
		//Marshal the response body into a struct
		if err := json.NewDecoder(resp.Body).Decode(&globalControlPlane); err != nil {
			log.Fatal("Error decoding data:", err)
		}

	}
	return globalControlPlane

}

// function to call Kong Konnect API and create a zone in the specified global control plane
func createMeshZone(globalControlPlane GlobalControlPlane)  MeshZone {
	meshZoneResponse := MeshZone{}
	client := &http.Client{}
	reqBody, err := json.Marshal(map[string]string{
		"name": mesh_zone_name,
	})
	req, err := http.NewRequest(http.MethodPost, kong_api_endpoint+"/"+mesh_api_version+"/mesh/control-planes/"+globalControlPlane.ID+"/api/provision-zone", bytes.NewBuffer(reqBody))


	if err != nil {
		log.Fatal("Error creating request:", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personal_access_token)
	req.Header.Set("Accept", "application/json")


	resp, err := client.Do(req)
	
	if err != nil {
			log.Fatal("Error sending request:", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		log.Fatal("UnAuthorized, pls validate your PAT token and Konnect RBAC permissions")
	}

	if resp.StatusCode == 409 {
		// fmt.Println("Detected an existing Zone with the same name, skipping creation")
		log.Fatal("Detected an existing Zone with the same name, skipping creation. Stopping execution. The implementation for this is pending. Please create the Secrets Manually.")
	}
	if resp.StatusCode == 200 {
		fmt.Println("Zone created")
		var createMeshZoneResponse CreateMeshZoneResponse
		err = json.NewDecoder(resp.Body).Decode(&createMeshZoneResponse)
		if err != nil {
			// fmt.Println("Error decoding data:", err)
			log.Fatal("Error decoding data:", err)
		}

		meshZoneResponse.Token = createMeshZoneResponse.Token
		meshZoneResponse.Name = mesh_zone_name
	}

	return meshZoneResponse

}