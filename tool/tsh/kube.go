/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

type kubeCommands struct {
	credentials *kubeCredentialsCommand
}

func newKubeCommand(app *kingpin.Application) kubeCommands {
	// TODO(awly): unhide this when other subcommends are implemented.
	kube := app.Command("kube", "Manage available kubernetes clusters").Hidden()
	var cmds kubeCommands

	cmds.credentials = &kubeCredentialsCommand{
		// This command is always hidden. It's called from the kubeconfig that
		// tsh generates and never by users directly.
		CmdClause: kube.Command("credentials", "Get credentials for kubectl access").Hidden(),
	}
	cmds.credentials.Flag("kube-cluster", "Name of the kubernetes cluster to get credentials for.").StringVar(&cmds.credentials.kubeCluster)

	return cmds
}

type kubeCredentialsCommand struct {
	*kingpin.CmdClause
	kubeCluster string
}

func newKubeCredentialsCommand(parent *kingpin.CmdClause) *kubeCredentialsCommand {
	c := &kubeCredentialsCommand{
		// This command is always hidden. It's called from the kubeconfig that
		// tsh generates and never by users directly.
		CmdClause: parent.Command("credentials", "Get credentials for kubectl access").Hidden(),
	}
	c.Flag("kube-cluster", "Name of the kubernetes cluster to get credentials for.").Required().StringVar(&c.kubeCluster)
	return c
}

func (c *kubeCredentialsCommand) run(cf *CLIConf) error {
	if c.kubeCluster == "" {
		return trace.BadParameter("missing the required --kube-cluster flag.")
	}
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(awly): use existing cert if possible.

	// TODO(awly): re-login if existing cert expired and running interactive.

	if err := tc.ReissueUserCerts(cf.Context, client.ReissueParams{
		RouteToCluster:    cf.SiteName,
		KubernetesCluster: c.kubeCluster,
	}); err != nil {
		return trace.Wrap(err)
	}

	// TODO(awly): store the new cert on disk

	k, err := tc.LocalAgent().GetKey()
	if err != nil {
		return trace.Wrap(err)
	}

	return c.writeResponse(k)
}

func (c *kubeCredentialsCommand) writeResponse(key *client.Key) error {
	crt, err := key.TLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			// Indicate  slightly earlier expiration to avoid the cert expiring
			// mid-request.
			ExpirationTimestamp:   &metav1.Time{Time: crt.NotAfter.Add(-1 * time.Minute)},
			ClientCertificateData: string(key.TLSCert),
			ClientKeyData:         string(key.Priv),
		},
	}
	data, err := runtime.Encode(kubeCodecs.LegacyCodec(kubeGroupVersion), resp)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(string(data))
	return nil
}

// Required magic boilerplate to use the k8s encoder.

var (
	kubeScheme       = runtime.NewScheme()
	kubeCodecs       = serializer.NewCodecFactory(kubeScheme)
	kubeGroupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1beta1",
	}
)

func init() {
	metav1.AddToGroupVersion(kubeScheme, schema.GroupVersion{Version: "v1"})
	clientauthv1beta1.AddToScheme(kubeScheme)
	clientauthentication.AddToScheme(kubeScheme)
}
