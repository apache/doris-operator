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

	if jdk17Opts, exists := javaOpts["java_opts_for_jdk_17"]; exists {
		jdk17OptsString := strings.ReplaceAll(jdk17Opts.(string), "\"", "")
		for _, opt := range strings.Split(jdk17OptsString, " ") {
			if strings.Contains(opt, krb5Property) {
				split := strings.Split(opt, "=")
				return split[len(split)-1]
			}
		}
	}

	if commonOpts, exists := javaOpts["java_opts"]; exists {
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
