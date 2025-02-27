// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package kerberos

import "strings"

const (
	KRB5_DEFAULT_CONFIG = "/etc/krb5.conf"
)

// GetKrb5ConfFromJavaOpts extracts the path to the Kerberos configuration file from the Java configuration options
// This function searches for configuration values in java.security.krb5.conf in the following order of priority:
// 1. First search for the configuration in JAVA_OPTS_FOR_JDK_17 (JDK17 specific configuration)
// 2. If not found, search for the general JAVA_OPTS configuration
// 3. If none is found, return the default value "/etc/krb5.conf"
// This behavior is documented: https://doris.apache.org/docs/3.0/lakehouse/datalake-analytics/hive?_highlight=kerberos_krb5_conf_path#connect-to-kerberos-enabled-hive
func GetKrb5ConfFromJavaOpts(javaOpts map[string]interface{}) string {
	krb5Property := "-Djava.security.krb5.conf="

	// All keys in the parsed config map are lowercase, so 'java_opts_for_jdk_17' is used here instead of 'JAVA_OPTS_FOR_JDK_17'.
	if jdk17Opts, exists := javaOpts["java_opts_for_jdk_17"]; exists {
		//  The jvm configuration value in the configuration file(fe.conf/be.conf) has  "  symbol, so it needs to be cleared
		jdk17OptsString := strings.ReplaceAll(jdk17Opts.(string), "\"", "")
		for _, opt := range strings.Split(jdk17OptsString, " ") {
			if strings.Contains(opt, krb5Property) {
				split := strings.Split(opt, "=")
				return split[len(split)-1]
			}
		}
	}

	// All keys in the parsed config map are lowercase, so 'java_opts' is used here instead of 'JAVA_OPTS'.
	if commonOpts, exists := javaOpts["java_opts"]; exists {
		//  The jvm configuration value in the configuration file(fe.conf/be.conf) has  "  symbol, so it needs to be cleared
		commonOptsString := strings.ReplaceAll(commonOpts.(string), "\"", "")
		for _, opt := range strings.Split(commonOptsString, " ") {
			if strings.Contains(opt, krb5Property) {
				split := strings.Split(opt, "=")
				return split[len(split)-1]
			}
		}
	}

	return KRB5_DEFAULT_CONFIG
}
