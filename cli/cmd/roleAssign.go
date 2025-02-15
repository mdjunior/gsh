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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/globocom/gsh/cli/cmd/auth"
	"github.com/globocom/gsh/cli/cmd/config"
	"github.com/gosimple/slug"
	"github.com/spf13/cobra"
)

// roleAssignCmd represents the roleAssign command
var roleAssignCmd = &cobra.Command{
	Use:   "role-assign [role] [user]",
	Short: "Associate a role to a user",
	Long: `

Assign a previous created role to a user.
	`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {

		// Get current target
		currentTarget := config.GetCurrentTarget()

		// Validate if ID is slug string
		if !slug.IsSlug(args[0]) {
			fmt.Printf("Client error parsing id, is it a slug string?: (%v)\n", args[0])
			os.Exit(1)
		}

		// Get OIDC HTTP Client
		oauth2Token, err := auth.RecoverToken(currentTarget)
		if err != nil {
			fmt.Printf("Client error getting http client: (%s)\n", err.Error())
			os.Exit(1)
		}

		// Setting custom HTTP client with timeouts
		var netTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		var netClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: netTransport,
		}

		// Make GSH request
		req, err := http.NewRequest("POST", currentTarget.Endpoint+"/authz/roles/"+args[0]+"/"+args[1], nil)
		if err != nil {
			fmt.Printf("Client pre post role request: (%s)\n", err.Error())
			os.Exit(1)
		}
		req.Header.Set("Authorization", "JWT "+oauth2Token.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := netClient.Do(req)
		if err != nil {
			fmt.Printf("Client error post role request: (%s)\n", err.Error())
			os.Exit(1)
		}

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Client error reading role response: (%s)\n", err.Error())
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Client error checking http status response: (%v)\n", resp.StatusCode)
		}
		defer resp.Body.Close()

		// Parse role response
		type RoleResponse struct {
			Details string `json:"details"`
			Message string `json:"message"`
			Result  string `json:"result"`
		}

		roleResponse := new(RoleResponse)
		if err := json.Unmarshal(body, &roleResponse); err != nil {
			fmt.Printf("Client error parsing role response: (%s)\n", err.Error())
			os.Exit(1)
		}

		if roleResponse.Result == "fail" {
			fmt.Printf("Client error calling GSH API: (%v)\n", roleResponse)
			os.Exit(1)
		}
		fmt.Println(roleResponse.Message)
	},
}

func init() {
	rootCmd.AddCommand(roleAssignCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// roleAssignCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// roleAssignCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
