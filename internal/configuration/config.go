//
// Copyright (c) 2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package configuration

// Configuration holds the config (e.g. namespace, daemonsetName) for the
// service.
type Configuration struct {
	DaemonsetName        string
	Namespace            string
	Images               map[string]string
	ImpersonateUsers     []string
	ServiceAccountID     string
	ServiceAccountSecret string
	OidcProvider         string
	ProxyURL             string
	CachingInterval      int
}

// Config stores the configuration from env vars
var Config Configuration

func init() {
	Config = Configuration{
		DaemonsetName:        getEnvVarOrExit(daemonsetNameEnvVar),
		Namespace:            getEnvVarOrExit(namespaceEnvVar),
		Images:               processImagesEnvVar(),
		ImpersonateUsers:     processImpersonateUsers(),
		ServiceAccountID:     getEnvVarOrExit(serviceAccountIDEnvVar),
		ServiceAccountSecret: getEnvVarOrExit(serviceAccountSecretEnvVar),
		OidcProvider:         getEnvVarOrExit(oidcProviderEnvVar),
		ProxyURL:             getEnvVarOrExit(proxyURLEnvVar),
		CachingInterval:      getCachingInterval(),
	}
}
