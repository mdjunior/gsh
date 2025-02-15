package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Init configure and check environment configuration
func Init() viper.Viper {
	// Configure defaults
	if len(os.Getenv("PORT")) == 0 {
		err := os.Setenv("PORT", "8000")
		if err != nil {
			fmt.Println("Error setting PORT environment variable")
		}
	}
	config := viper.New()
	config.SetConfigType("json")
	config.SetConfigName("config")
	config.AddConfigPath(".")
	err := config.ReadInConfig() // Find and read the config file
	if err != nil {              // Handle errors reading the config file
		fmt.Println("Config file not set, using .env variables")
	}
	config.SetDefault("storage_uri", "user:pass@tcp(localhost:3306)/gsh?charset=utf8&parseTime=True&multiStatements=true")
	config.SetDefault("oidc_callback_port", "30000")
	config.SetEnvPrefix("GSH")
	config.AutomaticEnv()
	return *config
}

// Check verify configuration
func Check(config viper.Viper) error {
	var fails uint

	// Check envs
	if len(os.Getenv("PORT")) == 0 {
		fmt.Printf("Environment variable PORT not defined\n")
		fails++
	}
	// Check Storage (MySQL)
	if len(config.GetString("storage_driver")) == 0 {
		fmt.Println("Storage driver (storage_driver) not set")
		fails++
	}
	if len(config.GetString("storage_uri")) == 0 {
		fmt.Println("Storage URI (storage_uri) not set")
		fails++
	}

	// Check CA
	if config.GetBool("ca_external") {
		if len(config.GetString("ca_signer_url")) == 0 {
			fmt.Println("CA signer URL (ca_signer_url) not set")
			fails++
		}
		if len(config.GetString("ca_public_key_url")) == 0 {
			fmt.Println("CA public key URL (ca_public_key_url) not set")
			fails++
		}
		if len(config.GetString("ca_endpoint")) == 0 {
			fmt.Println("CA endpoint (ca_endpoint) not set")
			fails++
		}
		if len(config.GetString("ca_role_id")) == 0 {
			fmt.Println("CA role ID (ca_role_id) not set")
			fails++
		}
		if len(config.GetString("ca_external_secret_id")) == 0 {
			fmt.Println("CA external (Vault) secret ID (ca_external_secret_id) not set")
			fails++
		}
	} else {
		if len(config.GetString("ca_private_key")) == 0 {
			fmt.Println("CA private key (ca_private_key) not set")
			fails++
		}
		if len(config.GetString("ca_public_key")) == 0 {
			fmt.Println("CA public key (ca_public_key) not set")
			fails++
		}
	}

	// Check OIDC
	if len(config.GetString("oidc_base_url")) == 0 {
		fmt.Println("OIDC base URL (oidc_base_url) not set")
		fails++
	}
	if len(config.GetString("oidc_realm")) == 0 {
		fmt.Println("OIDC realm (oidc_realm) not set")
		fails++
	}
	if len(config.GetString("oidc_audience")) == 0 {
		fmt.Println("OIDC audience or client id (oidc_audience) not set")
		fails++
	}
	if len(config.GetString("oidc_authorized_party")) == 0 {
		fmt.Println("OIDC authorized party or client id (oidc_authorized_party) not set")
		fails++
	}
	if len(config.GetString("oidc_claim")) == 0 {
		fmt.Println("OIDC claim (oidc_claim) not set")
		fails++
	}
	if len(config.GetString("oidc_claim_name")) == 0 {
		fmt.Println("OIDC claim name (oidc_claim_name) not set")
		fails++
	}
	if len(config.GetString("oidc_issuer")) == 0 {
		fmt.Println("OIDC issuer (oidc_issuer) not set")
		fails++
	}
	if len(config.GetString("oidc_certs")) == 0 {
		fmt.Println("OIDC certs (oidc_certs) not set")
		fails++
	}

	// Check for admins
	if len(config.GetStringSlice("perm_admin")) == 0 {
		fmt.Println("Admin users (perm_admin) not configured")
		fails++
	}

	if fails > 0 {
		return errors.New("incorrect configuration")
	}

	return nil
}
