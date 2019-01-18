// Copyright © 2019 Globo.com
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors
//    may be used to endorse or promote products derived from this software
//    without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/globocom/gsh/cli/cmd/auth"
	"github.com/globocom/gsh/cli/cmd/config"
	"github.com/globocom/gsh/cli/cmd/files"
	"github.com/globocom/gsh/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// hostConnectCmd represents the hostConnect command
var hostConnectCmd = &cobra.Command{
	Use:   "host-connect",
	Short: "Opens a remote shell inside host, using SSH certificates",
	Long: `Opens a remote shell inside host, using SSH certificates. You
can access an host just giving DNS name, or specifying the IP of the host.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get current target
		currentTarget := new(types.Target)
		targets := viper.GetStringMap("targets")
		for k, v := range targets {
			target := v.(map[string]interface{})

			// format output for activated target
			if target["current"].(bool) {
				currentTarget.Label = k
				currentTarget.Endpoint = target["endpoint"].(string)
			}
		}

		// Keys struct for reuse
		type Keys struct {
			SSHPublicKey  string
			SSHPrivateKey string
		}
		keys := new(Keys)

		// Get flags for SSH key type
		keyType, err := cmd.Flags().GetString("key-type")
		if err != nil {
			fmt.Printf("Client error parsing key-type option: (%s)\n", err.Error())
			os.Exit(1)
		}
		switch keyType {
		// RSA Keys
		case "rsa":
			// Generate keys
			privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				fmt.Printf("Client error generating RSA keys: (%s)\n", err.Error())
				os.Exit(1)
			}
			// convert publick key to SSH format
			pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
			if err != nil {
				fmt.Printf("Client error converting RSA to SSH keys: (%s)\n", err.Error())
				os.Exit(1)
			}
			keys.SSHPublicKey = string(ssh.MarshalAuthorizedKey(pub))

			// convert RSA private key to PEM format
			privateKeyPEM := &pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
			}
			keys.SSHPrivateKey = string(pem.EncodeToMemory(privateKeyPEM))
		}

		// Parse URL
		u, err := url.Parse(currentTarget.Endpoint)
		if err != nil {
			fmt.Printf("Client error parsing URL endpoint: (%s)\n", err.Error())
			os.Exit(1)
		}

		// Get preferred outbound ip of this machine
		conn, err := net.Dial("tcp", u.Host)
		if err != nil {
			conn, err = net.Dial("tcp", u.Host+":"+u.Scheme)
			if err != nil {
				fmt.Printf("Client error connecting on endpoint: (%s)\n", err.Error())
				os.Exit(1)
			}
		}
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.TCPAddr)

		// Make GSH API discovery
		configResponse, err := config.Discovery()
		if err != nil {
			fmt.Printf("GSH client discover error: %s\n", err.Error())
			os.Exit(1)
		}

		// Get OIDC HTTP Client
		oauth2Token, err := auth.RecoverToken(currentTarget)
		if err != nil {
			fmt.Printf("Client error getting http client: (%s)\n", err.Error())
			os.Exit(1)
		}

		// Get provider for username discovery
		ctx := context.Background()
		oauth2provider, err := oidc.NewProvider(ctx, configResponse.BaseURL+"/"+configResponse.Realm)
		if err != nil {
			fmt.Printf("Client error getting OIDC Provider: (%s)\n", err.Error())
			os.Exit(1)
		}
		// Get info about user
		var username string
		if !cmd.Flags().Changed("username") {
			userInfo, err := oauth2provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
			if err != nil {
				fmt.Printf("Client error getting OIDC userinfo: (%s)\n", err.Error())
				os.Exit(1)
			}
			claims := map[string]string{}
			userInfo.Claims(&claims)

			// Set username
			username = claims[configResponse.UsernameClaim]
			if username == "" {
				userLocal, err := user.Current()
				if err != nil {
					fmt.Printf("Client error getting username: (%s)\n", err.Error())
					os.Exit(1)
				}
				username = userLocal.Username
			}
		} else {
			username, err = cmd.Flags().GetString("username")
			if err != nil {
				fmt.Printf("Client error getting username: (%s)\n", err.Error())
				os.Exit(1)
			}
		}

		// prepare JSON to gsh api
		certRequest := types.CertRequest{
			Key:        keys.SSHPublicKey,
			RemoteHost: args[0],
			RemoteUser: username,
			UserIP:     localAddr.IP.String(),
		}

		// Marshall certificate to JSON
		certRequestJSON, _ := json.Marshal(certRequest)

		// Setting custom HTTP client with timeouts
		var netTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: time.Second,
		}
		var netClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: netTransport,
		}

		// Make GSH request
		req, err := http.NewRequest("POST", currentTarget.Endpoint+"/certificates", bytes.NewBuffer(certRequestJSON))
		req.Header.Set("Authorization", "JWT "+oauth2Token.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := netClient.Do(req)
		if err != nil {
			fmt.Printf("Client error post certificate request: (%s)\n", err.Error())
			os.Exit(1)
		}

		// Read body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Client error reading certificate response: (%s)\n", err.Error())
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Client error checking http status response: (%v)\n", resp.StatusCode)
			os.Exit(1)
		}
		defer resp.Body.Close()

		// Parse certificate response
		type CertResponse struct {
			Certificate string `json:"certificate"`
			Result      string `json:"result"`
		}
		certResponse := new(CertResponse)
		if err := json.Unmarshal(body, &certResponse); err != nil {
			fmt.Printf("Client error parsing certificate response: (%s)\n", err.Error())
			os.Exit(1)
		}
		// certificate at certResponse.Certificate

		// Write files
		keyFile, certFile, err := files.WriteKeys(keys.SSHPrivateKey, certResponse.Certificate)
		if err != nil {
			fmt.Printf("Client error write certificate files: (%s)\n", err.Error())
			os.Exit(1)
		}

		sh := exec.Command("ssh", "-i", keyFile, "-i", certFile, "-l", username, args[0])
		sh.Stdout = os.Stdout
		sh.Stdin = os.Stdin
		sh.Stderr = os.Stderr
		sh.Run()

		fmt.Println("host-connect called")
	},
}

func init() {
	rootCmd.AddCommand(hostConnectCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// hostConnectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// hostConnectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	hostConnectCmd.Flags().StringP("key-type", "A", "rsa", "Defines type of auto generated ssh key pair (rsa)")
	hostConnectCmd.Flags().StringP("username", "u", "from OIDC token", "Defines remote user used on remote host")
}
