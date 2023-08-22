package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	clustertype "kong-konnect-runtime-cert-generator/enums"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

var (
	kong_api_endpoint     string
	personal_access_token string
	api_version           string
	runtime_group_name    string
	cluster_type		  string
)

type RunTimeGroupName struct {
	Name string `json:"name"`
	ClusterType string `json:"cluster_type"`
}

type RunTimeGroupId struct {
	Id string `json:"id"`
}

type RuntimeConfiguration struct {
	Meta struct {
		Page struct {
			Total  int `json:"total"`
			Size   int `json:"size"`
			Number int `json:"number"`
		} `json:"page"`
	} `json:"meta"`
	Data []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Labels      struct {
		} `json:"labels"`
		Config struct {
			ControlPlaneEndpoint string `json:"control_plane_endpoint"`
			TelemetryEndpoint    string `json:"telemetry_endpoint"`
			ClusterType          string `json:"cluster_type"`
		} `json:"config"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"data"`
}

type Output struct {
	ControlPlaneEndpoint string `json:"cluster_dns"`
	TelemetryEndpoint    string `json:"telemetry_dns"`
	Name                 string `json:"runtime_name"`
	AWSCertManagerCRT    string `json:"cert_secret_name"`
	AWSCertManagerKey    string `json:"key_secret_name"`
	PersonalAccessToken  string `json:"personal_access_token"`
}

// Create a struct containing certificate
type Cert struct {
	Certificate string `json:"cert"`
}

func main() {
	flag.StringVar(&kong_api_endpoint, "api-endpoint", "https://us.api.konghq.com", "Kong API endpoint")
	flag.StringVar(&api_version, "api-version", "v2", "Kong API version")
	flag.StringVar(&personal_access_token, "personal-access-token", "", "Kong Personal Access Token")
	flag.StringVar(&runtime_group_name, "runtime-group-name", "default", "Runtime group name")
	flag.StringVar(&cluster_type, "cluster-type", string(clustertype.ClusterTypeHybrid), "Cluster type")
	flag.Parse()

	// check if cluster type is valid enum
	if cluster_type != string(clustertype.ClusterTypeHybrid) && cluster_type != string(clustertype.ClusterTypeKiC) && cluster_type != string(clustertype.ClusterTypeComposite) {
		log.Fatal("Invalid cluster type, please use one of the following: CLUSTER_TYPE_HYBRID, CLUSTER_TYPE_K8S_INGRESS_CONTROLLER, CLUSTER_TYPE_COMPOSITE")
	}

	data := RunTimeGroupName{
		Name: runtime_group_name,
		ClusterType: cluster_type,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, kong_api_endpoint+"/"+api_version+"/runtime-groups", bytes.NewBuffer(jsonData))

	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personal_access_token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		log.Fatal("UnAuthorized, pls validate your PAT token and Konnect RBAC permissions")
	}

	if resp.StatusCode == 409 {
		fmt.Println("Detected existing runtime group, skipping creation")
	}
	if resp.StatusCode == 201 {
		fmt.Println("Runtime group created")
	}

	GenerateKeys()

}

// function to call kong api and get the runtime group id
func GetRuntimeGroupConfiguration() RuntimeConfiguration {

	filter := map[string]string{
		"filter[name][eq]": runtime_group_name,
	}
	filterValues := url.Values{}
	for k, v := range filter {
		filterValues.Add(k, v)
	}
	filterData := filterValues.Encode()
	get_runtime_groups := kong_api_endpoint + "/" + api_version + "/runtime-groups" + "?" + filterData

	req, err := http.NewRequest(http.MethodGet, get_runtime_groups, nil)

	if err != nil {
		fmt.Println("Error creating request:", err)
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personal_access_token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		panic(err)
	}
	defer resp.Body.Close()

	var response RuntimeConfiguration
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		fmt.Println("Error decoding data:", err)
		panic(err)
	}
	return response
}

// function to generate public keys and private keys for the runtime group
func GenerateKeys() {

	var runtime_configuration RuntimeConfiguration
	runtime_configuration = GetRuntimeGroupConfiguration()
	fmt.Println("Runtime Group ID: ", runtime_configuration.Data[0].ID)

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		fmt.Println("Error generating private key:", err)
		panic(err)
	}

	// create a template for self signed certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Kong"},
			CommonName:   "Kong",
			Country:      []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		fmt.Println("Error creating certificate:", err)
		panic(err)

	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Fill in the struct containing the certificate and private key
	var keyPair Cert
	keyPair.Certificate = string(certPEM)
	// keyPair.PrivateKey = string(privateKeyPEM)

	// Marshall the struct to json
	jsonData, err := json.Marshal(keyPair)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		panic(err)

	}

	req, err := http.NewRequest(http.MethodPost, kong_api_endpoint+"/"+api_version+"/runtime-groups/"+runtime_configuration.Data[0].ID+"/dp-client-certificates", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personal_access_token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	//TODO: Log Levels -  fmt.Println("Certificate Upload status:", resp.Status)

	var output Output
	output.ControlPlaneEndpoint = runtime_configuration.Data[0].Config.ControlPlaneEndpoint
	output.TelemetryEndpoint = runtime_configuration.Data[0].Config.TelemetryEndpoint
	output.Name = runtime_configuration.Data[0].Name
	output.AWSCertManagerCRT = runtime_configuration.Data[0].ID + "-cert"
	output.AWSCertManagerKey = runtime_configuration.Data[0].ID + "-key"
	output.PersonalAccessToken = runtime_configuration.Data[0].ID + "-pat-token"

	CreateSecret(output.AWSCertManagerCRT, string(certPEM))
	CreateSecret(output.AWSCertManagerKey, string(privateKeyPEM))
	CreateSecret(output.PersonalAccessToken, personal_access_token)

	//convert struct to JSON

	outputJson, _ := json.Marshal(output)
	fmt.Printf("%s\n", outputJson)
}

// Function to create secrets in AWS Secrets Manager
func CreateSecret(name string, secret string) {
	// Set the name of the secret to create and delete
	secretName := name

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("failed to load AWS config, " + err.Error())
	}

	// Create the Secrets Manager client
	svc := secretsmanager.NewFromConfig(cfg)

	// Try to describe the secret to check if it exists
	_, err = svc.DescribeSecret(context.Background(), &secretsmanager.DescribeSecretInput{
		SecretId: &secretName,
	})

	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			// Secret doesn't exist, proceed to create new secret
			//TODO: Log Levels -  fmt.Println("Secret not found, proceeding to create new secret...")

		} else {
			// Handle other errors as needed
			panic("failed to describe secret, " + err.Error())
		}
	} else {
		// Secret exists, force delete the secret
		//TODO: Log Levels -  fmt.Printf("Secret %s found, forcing deletion...\n", secretName)
		isTrue := true
		_, err = svc.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   &secretName,
			ForceDeleteWithoutRecovery: &isTrue,
		})
		if err != nil {
			panic("failed to force delete secret, " + err.Error())
		}
		// Wait for the deletion to complete
		for {
			time.Sleep(5 * time.Second)
			_, err = svc.DescribeSecret(context.Background(), &secretsmanager.DescribeSecretInput{
				SecretId: &secretName,
			})
			if err != nil {
				if strings.Contains(err.Error(), "ResourceNotFoundException") {
					// Deletion is complete, proceed to create new secret
					//TODO: Log Levels -  fmt.Println("Deletion is complete, proceeding to create new secret...")
					break
				} else {
					// Handle other errors as needed
					panic("failed to describe secret, " + err.Error())
				}
			} else {
				//TODO : Debug Loggingfmt.Println("Secret still exists, waiting for deletion to complete...")
			}
		}

	}

	// Create the new secret
	//TODO : Debug Loggingfmt.Printf("Creating new secret with name %s \n", secretName)
	_, err = svc.CreateSecret(context.Background(), &secretsmanager.CreateSecretInput{
		Name:         &secretName,
		SecretString: &(secret),
	})
	if err != nil {
		panic("failed to create secret, " + err.Error())
	}
	//TODO : Debug Logging fmt.Printf("Secret %s created with %s\n", *createOutput.Name, *createOutput.ARN)
}
