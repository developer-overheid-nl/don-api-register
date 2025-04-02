# Go API client for don_api_register

API for retrieveing all the API's and repositories on Developer Overheid

## Overview
This API client was generated by the [OpenAPI Generator](https://openapi-generator.tech) project.  By using the [OpenAPI-spec](https://www.openapis.org/) from a remote server, you can easily generate an API client.

- API version: dev
- Package version: 1.0.0
- Generator version: 7.12.0
- Build package: org.openapitools.codegen.languages.GoClientCodegen

## Installation

Install the following dependencies:

```sh
go get github.com/stretchr/testify/assert
go get golang.org/x/net/context
```

Put the package under your project folder and add the following in import:

```go
import don_api_register "github.com/GIT_USER_ID/GIT_REPO_ID"
```

To use a proxy, set the environment variable `HTTP_PROXY`:

```go
os.Setenv("HTTP_PROXY", "http://proxy_name:proxy_port")
```

## Configuration of Server URL

Default configuration comes with `Servers` field that contains server objects as defined in the OpenAPI specification.

### Select Server Configuration

For using other server than the one defined on index 0 set context value `don_api_register.ContextServerIndex` of type `int`.

```go
ctx := context.WithValue(context.Background(), don_api_register.ContextServerIndex, 1)
```

### Templated Server URL

Templated server URL is formatted using default variables from configuration or from context value `don_api_register.ContextServerVariables` of type `map[string]string`.

```go
ctx := context.WithValue(context.Background(), don_api_register.ContextServerVariables, map[string]string{
	"basePath": "v2",
})
```

Note, enum values are always validated and all unused variables are silently ignored.

### URLs Configuration per Operation

Each operation can use different server URL defined using `OperationServers` map in the `Configuration`.
An operation is uniquely identified by `"{classname}Service.{nickname}"` string.
Similar rules for overriding default operation server index and variables applies by using `don_api_register.ContextOperationServerIndices` and `don_api_register.ContextOperationServerVariables` context maps.

```go
ctx := context.WithValue(context.Background(), don_api_register.ContextOperationServerIndices, map[string]int{
	"{classname}Service.{nickname}": 2,
})
ctx = context.WithValue(context.Background(), don_api_register.ContextOperationServerVariables, map[string]map[string]string{
	"{classname}Service.{nickname}": {
		"port": "8443",
	},
})
```

## Documentation for API Endpoints

All URIs are relative to *http://localhost*

Class | Method | HTTP request | Description
------------ | ------------- | ------------- | -------------
*APIAPI* | [**GetAPI**](docs/APIAPI.md#getapi) | **Get** /apis/{id} | Get API
*APIAPI* | [**ListAPI**](docs/APIAPI.md#listapi) | **Get** /apis | List API&#39;s
*RepositoryAPI* | [**ListRepository**](docs/RepositoryAPI.md#listrepository) | **Get** /repositories | List repositories


## Documentation For Models

 - [APIListItem](docs/APIListItem.md)
 - [APIListItemContact](docs/APIListItemContact.md)
 - [APIListItemEnvironmentsInner](docs/APIListItemEnvironmentsInner.md)
 - [APIListItemOrganization](docs/APIListItemOrganization.md)
 - [APIListItemTermsOfUse](docs/APIListItemTermsOfUse.md)
 - [InlineObject](docs/InlineObject.md)
 - [RepositoryListItem](docs/RepositoryListItem.md)


## Documentation For Authorization

Endpoints do not require authorization.


## Documentation for Utility Methods

Due to the fact that model structure members are all pointers, this package contains
a number of utility functions to easily obtain pointers to values of basic types.
Each of these functions takes a value of the given basic type and returns a pointer to it:

* `PtrBool`
* `PtrInt`
* `PtrInt32`
* `PtrInt64`
* `PtrFloat`
* `PtrFloat32`
* `PtrFloat64`
* `PtrString`
* `PtrTime`

## Author



