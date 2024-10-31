# Configuring the Username and Password for the Management Cluster
The management of Doris nodes requires connecting to the live FE nodes via the MySQL protocol using a username and password for operations. Doris implements [a permission management mechanism similar to RBAC](https://doris.apache.org/zh-CN/docs/admin-manual/auth/authentication-and-authorization?_highlight=rbac#doris-%E5%86%85%E7%BD%AE%E7%9A%84%E9%89%B4%E6%9D%83%E6%96%B9%E6%A1%88), and the management of nodes requires the user to have the [Node_priv](https://doris.apache.org/zh-CN/docs/admin-manual/auth/authentication-and-authorization#%E6%9D%83%E9%99%90%E7%B1%BB%E5%9E%8B) permission. By default, Doris Operator deploys and manages the cluster configured with DorisCluster resources using the root user with all permissions in passwordless mode. After adding a password to the root user, it is necessary to explicitly configure the username and password with Node_Priv permission in the DorisCluster resource, so that Doris Operator can perform automated management operations on the cluster.  
DorisCluster resources provide two ways to configure the username and password required for managing cluster nodes, including:   
- the way of environment variable configuration and the way of using Secret.   
- Configuring the username and password for cluster management can be divided into three cases: initializing the root user password during cluster deployment;   
- automatically setting a non-root user with management permissions in the root passwordless deployment; setting the root user password after deploying the cluster in root passwordless mode.    
## Configuring the Root User Password during Cluster Deployment
Doris supports configuring the root user's password in encrypted form in fe.conf. To configure the root user's password during the first deployment of Doris, follow these steps so that Doris Operator can automatically manage the cluster nodes:  
**1. Generate the Root Encrypted Password**  
Doris supports [setting the root user's password in the fe.conf](https://doris.apache.org/zh-CN/docs/admin-manual/config/fe-config?_highlight=initial_#initial_root_password) in encrypted form. The password encryption is implemented using two-stage SHA-1 encryption. The code implementation is as follows:  
Java Code for Two-Stage SHA-2 Encryption:  
```java
import org.apache.commons.codec.digest.DigestUtils;
public static void main( String[] args ) {
      //the original password
      String a = "123456";
      String b = DigestUtils.sha1Hex(DigestUtils.sha1(a.getBytes())).toUpperCase();
      //output the 2 stage encrypted password.
      System.out.println("*"+b);
  }
```
Golang Code for Two-Stage SHA-1 Encryption:  
```go
import (
"crypto/sha1"
"encoding/hex"
"fmt"
"strings"
)

func main() {
    //original password
    plan := "123456"
    //the first stage encryption.
    h := sha1.New()
    h.Write([]byte(plan))
    eb := h.Sum(nil)
    
    //the two stage encryption.
    h.Reset()
    h.Write(eb)
    teb := h.Sum(nil)
    dst := hex.EncodeToString(teb)
    tes := strings.ToUpper(fmt.Sprintf("%s", dst))
    //output the 2 stage encrypted password. 
    fmt.Println("*"+tes)
}
```
Configure the encrypted password into fe.conf according to the requirements of the configuration file format. Then, distribute the configuration file to the k8s cluster in the form of a configmap according to the introduction in [the Cluster Parameter Configuration Section](https://doris.apache.org/zh-CN/docs/install/cluster-deployment/k8s-deploy/install-config-cluster#%E9%9B%86%E7%BE%A4%E5%8F%82%E6%95%B0%E9%85%8D%E7%BD%AE).  
**2. Configure the DorisCluster Resource**  
After setting the root initialization password in the configuration file, the root password will take effect immediately after the first Doris FE node starts. When other nodes join the cluster, Doris Operator needs to operate using the root username + password. It is necessary to specify the username + password in the deployed DorisCluster resource so that Doris Operator can automatically manage the cluster nodes.  
- Using Environment Variables  
    Configure the username root and password into the ".spec.adminUser.name" and ".spec.adminUser.password" fields in the DorisCluster resource. Doris Operator will automatically convert the following configuration into environment variables for the container to use. The auxiliary services inside the container will use the username and password configured by the environment variables to add themselves to the specified cluster. The configuration format is as follows:  
    ```yaml
    spec:
      adminUser:
        name: root
        password: ${password}
    ```
    Here, ${password} is the unencrypted password of root.  
- Using Secret:  
    Doris Operator provides the use of [Basic authentication Secret](https://kubernetes.io/docs/concepts/configuration/secret/#basic-authentication-secret) to specify the username and password of the management node. After the DorisCluster resource is configured to use the required Secret, Doris Operator will automatically mount the Secret to the specified location of the container in the form of a file. The auxiliary services of the container will parse the username and password from the file to automatically add themselves to the specified cluster. The stringData of basic-authentication-secret only contains two fields: username and password. The process of using Secret to configure the management username and password is as follows:  
    a. Configure the Required Secret  
    Configure the required Basic authentication Secret according to the following format:  
    ```yaml
    stringData:
      username: root
      password: ${password}
    ```
    Here, ${password} is the unencrypted password set for root.
b. Configure the DorisCluster Resource to be Deployed  
    Configure the DorisCluster to specify the required Secret in the following format:  
    ```yaml
    spec:
      authSecret: ${secretName}
    ```
    Here, ${secretName} is the name of the Secret containing the root username and password.  
## Automatically Creating Non-Root Management Users and Passwords during Deployment (Recommended)
During the first deployment, do not set the initialization password of root. Instead, set the non-root user and login password through the environment variable or using Secret. The auxiliary services of the Doris container will automatically create the configured user in the database, set the password, and grant the Node_priv permission. Doris Operator will manage the cluster nodes using the automatically created username and password.  
- Using Environment Variables:  
    Configure the DorisCluster resource to be deployed according to the following format:
    ```yaml
    spec:
      adminUser:
        name: ${DB_ADMIN_USER}
        password: ${DB_ADMIN_PASSWD}
    ```
    Here, ${DB_ADMIN_USER} is the newly created username, and ${DB_ADMIN_PASSWD} is the password set for the newly created username.  
- Using Secret:  
a. Configure the Required Secret  
    Configure the required Basic authentication Secret according to the following format:  
    ```yaml
    stringData:
      username: ${DB_ADMIN_USER}
      password: ${DB_ADMIN_PASSWD}
    ```
    Here, ${DB_ADMIN_USER} is the newly created username, and ${DB_ADMIN_PASSWD} is the password set for the newly created username.  
    Deploy the updated Secret to the k8s cluster by running `kubectl -n ${namespace} apply -f ${secretFileName}.yaml`. Here, ${namespace} is the namespace where the DorisCluster resource needs to be deployed, and ${secretFileName} is the file name of the Secret to be deployed.  
b. Configure the DorisCluster Resource Requiring Secret    
    Update the DorisCluster resource according to the following format:  
    ```yaml
    spec:
      authSecret: ${secretName}
    ```
    Here, ${secretName} is the name of the deployed Basic authentication Secret.  

:::tip Tip  
After deployment, please set the root password. Doris Operator will switch to using the automatically newly created username and password to manage the nodes. Please avoid deleting the automatically created user.  
:::
## Setting the Root User Password after Cluster Deployment
After the Doris cluster is deployed and the root user's password is set, it is necessary to configure a user with [Node_priv](https://doris.apache.org/zh-CN/docs/admin-manual/auth/authentication-and-authorization/#%E6%9D%83%E9%99%90%E7%B1%BB%E5%9E%8B) permission into the DorisCluster resource so that Doris Operator can automatically manage the cluster nodes. It is not recommended to use root as this username. Please refer to [the User Creation and Permission Assignment Section](https://doris.apache.org/zh-CN/docs/sql-manual/sql-statements/Account-Management-Statements/CREATE-USER) to create a new user and grant Node_priv permission. After creating the user, specify the new management user and password through the environment variable or Secret method, and configure the corresponding DorisCluster resource.  
**1. Create a User with Node_priv Permission**  
After connecting to the database using the MySQL protocol, use the following command to create a simple user with only Node_priv permission and set the password.  
```shell
CREATE USER '${DB_ADMIN_USER}' IDENTIFIED BY '${DB_ADMIN_PASSWD}';
```
Here, ${DB_ADMIN_USER} is the username you hope to create, and ${DB_ADMIN_PASSWD} is the password you hope to set for the newly created user.  
**2. Grant Node_priv Permission to the Newly Created User**  
After connecting to the database using the MySQL protocol, use the following command to grant Node_priv permission to the newly created user.  
```shell
GRANT NODE_PRIV ON *.*.* TO ${DB_ADMIN_USER};
```
Here, ${DB_ADMIN_USER} is the newly created username.  
For detailed usage of creating users, setting passwords, and granting permissions, please refer to the official document [CREATE-USER](https://doris.apache.org/zh-CN/docs/sql-manual/sql-statements/Account-Management-Statements/CREATE-USER/) section.  
**3. Configure DorisCluster**  
- Using Environment Variables  
    Configure the newly created username and password into the ".spec.adminUser.name" and ".spec.adminUser.password" fields in the DorisCluster resource. Doris Operator will automatically convert the following configuration into environment variables. The auxiliary services inside the container will use the username and password configured by the environment variables to add themselves to the specified cluster. The configuration format is as follows:  
    ```yaml
    spec:
      adminUser:
        name: ${DB_ADMIN_USER}
        password: ${DB_ADMIN_PASSWD}
    ```
    Here, ${DB_ADMIN_USER} is the newly created username, and ${DB_ADIC_PASSWD} is the password set for the newly created user.  
- Using Secret  
    a. Configure the Required Secret  
    Configure the required Basic authentication Secret according to the following format:  
    ```yaml
    stringData:
      username: ${DB_ADMIN_USER}
      password: ${DB_ADMIN_PASSWD}
    ```
    Here, ${DB_ADMIN_USER} is the newly created username, and ${DB_ADMIN_PASSWD} is the password set for the newly created username.  
    Deploy the configured Secret to the k8s cluster by running kubectl -n ${namespace} apply -f ${secretFileName}.yaml. Here, ${namespace} is the namespace where the DorisCluster resource needs to be deployed, and ${secretFileName} is the file name of the Secret to be deployed.  
b. Update the DorisCluster Resource Requiring Secret  
    Update the DorisCluster resource according to the following format:  
    ```yaml
    spec:
      authSecret: ${secretName}
    ```
    Here, ${secretName} is the name of the deployed Basic authentication Secret.  

:::tip Tip  
After setting the root password and configuring the new username and password for managing nodes after deployment, the existing services will be restarted once in a rolling manner.  
:::
