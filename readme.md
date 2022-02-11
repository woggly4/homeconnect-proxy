# Home Connect Proxy
The Home Connect Proxy application allows simplified use of all Home Connect APIs without need to take care for authenctication headers and access token refresh. Upon initial authentication, the proxy persists the OAUTH access token, its exiry period, and the refresh token. Prior any next call, if access token is expired, it is automatically refreshed before calling Home Connect.
From user's perspective no direct calls are made to Home Connect, but to the 'internal' endpoints of the proxy. 


## Supported APIs
The application design allows all Home Connect APIs to be used via the proxy, and in the way documented at https://apiclient.home-connect.com.
All proxy end points are listed when accessing ```http://proxy_uri:port```. 

    /
	/proxy/auth
	/proxy/auth/redirect
	/proxy/success
	/homeappliances
	/homeappliances/{.*}
	/homeappliances/{.*}/programs
	/homeappliances/{.*}/programs/available
	/homeappliances/{.*}/programs/available/{.*}
	/homeappliances/{.*}/programs/active
	/homeappliances/{.*}/programs/active/options
	/homeappliances/{.*}/programs/active/options/{.*}
	/homeappliances/{.*}/programs/selected
	/homeappliances/{.*}/programs/selected/options
	/homeappliances/{.*}/programs/selected/options/{.*}
	/homeappliances/{.*}/status
	/homeappliances/{.*}/status/{.*}
	/homeappliances/{.*}/images
	/homeappliances/{.*}/images/{.*}
	/homeappliances/{.*}/settings
	/homeappliances/{.*}/settings/{.*}
	/homeappliances/{.*}/commands
	/homeappliances/{.*}/commands/{.*}
	/homeappliances/{.*}/events
	/homeappliances/events

Endpoints under ```/homeappliances``` correspond to the Home Connect APIs. In addition to those ```/``` serves the list above, and the three routes under ```/proxy/``` are required for the initial authentication of the application.


## SSE event stream and MQTT publishing
Home Connect features [server sent events](https://api-docs.home-connect.com/events) stream with status updates about the device(s) using the endpoints ```/homeappliances/{.*}/events``` and ```/homeappliances/events```. 
The stream can be accessed via the respective endpoints of the proxy. Additionally, the proxy implements mechanism to publish all events to a specified MQTT broker. 


## Build
The intended way to run is in a Docker container and the Dockerfile to create its image is provided. 
To build, clone the repository and run ```docker build -t neterra-proxy .```
The app can also be compiled and run natively.

## Configuration
All configuration parameters are passed using environment variables as follows:

```CLIENT_ID```: Home Connect application client ID as registered at https://developer.home-connect.com/applications

```CLIENT_SECRET```: Application client secret as defined in the registration

```CLIENT_SCOPES```: Application authorization scopes as defined in the registration. Ref. https://api-docs.home-connect.com/authorization?#authorization-scopes for details. This should be space separated (escaped by %20) list of permissions.

```PORT```: TCP port within the docker container at which the proxy would be accessible. This parameter is optional, if not specified the default port 8088 would be used. Please note that the port should be also published to the docker host so proxy can be accessed from outside world.

```MQTT_HOST```: IP address or host of the MQTT server to publish SSE event stream to. Parameter is optional, in case not specified localhost is used.

```MQTT_PORT```: TCP port at which the MQTT broker is running. Parameter is optional, in case not specified, default MQTT port 1883 is used.

For monitoring a troubleshooting the application logfile can also be mapped using docker volume to the host file. Same is valid for the access token cache, which if persisted would prevent the need of reauthorisation if the docker container gets rebuilt. 
<font color="red">The access token cache is in plain text and persisting it outside of the container may feature security risk.</font>


### Sample run command
```
docker run \
	--name homeconnect-proxy \
    --net=bridge \
    -p 8088:8088 \
	-e CLIENT_ID=<client it> \
    -e CLIENT_SECRET=<client secret> \
    -e CLIENT_SCOPES=<client scope(s)> \
	-e MQTT_HOST=<mqtt broker> \
	-e TZ=Europe/Amsterdam \
	-d \
	-v <container folder>/app.log:/app.log \
	-v <container folder>/token.cache:/token.cache \
    --restart=always \
    homeconnect-proxy:latest
```

