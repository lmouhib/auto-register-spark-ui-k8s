![Docker Image Version](https://img.shields.io/docker/v/lmouhib/auto-register-spark-ui-k8s?label=Image&link=https%3A%2F%2Fhub.docker.com%2Fr%2Flmouhib%2Fauto-register-spark-ui-k8s)


# Auto Register K8s Spark UI

## Overview

Users submitting Apache Spark applications on Kubernetes often struggle to get access to Spark UI. If they are using Spark Operator they can setup the ingress through Spark Appliction CRD, however, when using `spark-submit` or other methods they need to have access to the Kubernetes cluster which is not always trivial and is a paper cut action this project aims to solve. Auto Register Spark UI for Apache Spark applications running Kubernetes is a light weight Kubernetes controller that provides a solution to automatically register Spark UIs running on Kubernetes with a central proxy server. It simplifies the management and access of Spark UIs by dynamically updating the proxy server with the Spark UI endpoints as users submit spark jobs or run session from Jupyter notebooks. 

## Features

- **Automatic Registration**: Automatically registers Spark UIs with a central proxy server.
- **Dynamic Updates**: Updates the proxy server with new Spark UI endpoints as they are created and remove them when the spark application finishes running.
- **Customizable**: Easily configurable to fit different deployment environments.

## How it works

The Auto Register Spark UI controller operates by utilizing a Kubernetes informer to listen for newly created services within the cluster. It specifically filters these services to identify those associated with Spark applications by checking for a predefined Spark application selector `spark-app-selector`. Once a matching service is detected, the controller dynamically creates an Ingress resource and add to it a path to expose the Spark UI endpoint. This Ingress is configured to route traffic to the Spark UI service, thereby making it accessible through the central proxy server. Additionally, the controller ensures that the Ingress and/or the path are removed when the Spark application completes, maintaining an up-to-date and clean environment. The controller create a path based on the name of the spark application or namespace where the spark application is submitted and spark application. The following is the URL format, `http://ngnix-ip-or-dns-name/spark-application-name/` or `http://ngnix-ip-or-dns-name/spark-namespace/spark-application-name/`.

## Prerequisites

- `kubectl` command-line tool
- Docker #optional
- Nginx Controller deployed in the Kubernete cluster.

## Installation

1. **Clone the repository**:

```shell
git clone https://github.com/lmouhib/auto-register-spark-ui-k8s.git
cd auto-register-spark-ui-k8s
```

2. **Deploy the Spark UI Auto Register Service**:

The deployment below use an image built and hosted in the following repository `lmouhib/auto-register-spark-ui-k8s
`, you can use the provided Dockerfile in this repository to build and deploy your own image.

```shell
kubectl apply -f auto_register_spark_ui_deployment.yaml
```

3. Customizing the controller behavior
The controller expose certain paramter to configure how the Spark UI path is constructed or how it is detected. 

    _**SPARK_LABEL_SERVICE_SELECTOR**_: The label selector used to identify Spark services. Default is "spark-app-selector".

    _**SPARK_NAMESPACE**_: The Kubernetes namespace where the Spark jobs are running. Default the controller will listen to all namespaces.

    _**NAMESPACED_INGRESS_PATH**_: Whether to use namespaced ingress paths. Default is "true".
        
    _**INGRESS_NAME**_: The name of the ingress resource to be created. Default is "spark-ui-ingress".

    _**INGRESS_TYPE**_: The ingress controller available in the cluster, only two are supported: `nginx` or `traefik`. 

    _**AUTHENTICATION_SETUP**_: The name of the secret that holds the authentication file for `nginx` or `traefik`. 

## Usage

### Ingress Controllers supported

* NGINX
    * In order for the controller to create and manage Ingress based on NGINX, you need to set the environment variable `INGRESS_TYPE` to `nginx`.
* Traefik

     * In order for the controller to create and manage Ingress based on Traefik, you need to set the environment variable `INGRESS_TYPE` to `traefik`.

    * When using `traefik` the controller will create a a `traefik Middlware` to strip the url and forward it to spark driver. In order manage the lifecycle of the middleware, _**you need to uncomment the in the cluster role the policy that allow to manage the Middlware.**_ Withouth it the controller will fail.

### Submitting a Spark Job

In order for the Spark UI to be exposed correctly, you need to configure the spark application to use the nginx proxy. Below you will find an example with `spark-submit`. Note, the `spark.ui.proxyRedirectUri` can be defined as default in the `spark-default` file.

```sh
    ./bin/spark-submit \
    --master k8s://https://x.x.x.x \
    --deploy-mode cluster \
    --name pi \
    --conf spark.executor.instances=2 \
    --conf spark.kubernetes.container.image=spark:latest \
    --conf spark.kubernetes.authenticate.driver.serviceAccountName=spark \
    --conf spark.ui.proxyRedirectUri=http://ngnix-ip-or-dns-name \
    --conf spark.ui.proxyBase=/pi \
    local:///opt/spark/examples/src/main/python/pi.py 1000000
```


_Note_: You can get `proxyRedirectUri` by running the command: `kubectl  get svc ingress-nginx-controller -n ingress-nginx`.
The command assume you deployed `nginx` in the following namespace `ingress-nginx`, and the name of `nginx` service of type `LOAD BALANCER` is `ingress-nginx-controller`
and will provide an `EXTERNAL-IP` that should be used as `proxyRedirectUri`.

The Spark UI for the application submitted above would be in the following address : `http://ngnix-ip-or-dns-name/pi/`.

### Setting up authentication

To set up authentication, follow these steps:

1. **Create a password file using `htpasswd`:**

```sh
htpasswd -c auth <username>
```
You will be prompted to enter and confirm a password for the specified username. The command will create a file name `auth`.

2. Create a Kubernetes secret:

Once the password file is created, you need to create a Kubernetes secret to store the authentication data. Use the following command to create the secret, ensuring the key is named auth:

```sh
kubectl create secret generic spark-ui-auth --from-file=auth=./auth
```
if you change the name of the secret make sure to change also its name in the `RBAC` as well as in the environment variable `AUTHENTICATION_SETUP` in the file `auto_register_spark_ui_deployment.yaml`.

3. Deploy the controller, when a new ingress is created by the conroller, it will add the necessary annotation as well as the create the middleware for traefik to use the sercret that contains the list of username and password you created in step 1 and 2.

### Demo

<img src="https://github.com/lmouhib/auto-register-spark-ui-k8s/blob/main/assets/demo.gif" width="600" alt="Demo gif">


# Improvements

* Currently this controller work only with `NGINX`, one improvement is to make it generic and to work with any ingress that suport URL rewrite or prefix stripping.
* Add unit tests
* Provide build instructions to push image to Image repository for cloud providers.
* Refactor the controller to use configmap and not environment variables. 
