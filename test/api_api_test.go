/*
Developer Overheid API

Testing APIAPIService

*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech);

package don_api_register

import (
	"context"
	"testing"

	openapiclient "github.com/developer-overheid-nl/don-api-register"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_don_api_register_APIAPIService(t *testing.T) {

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)

	t.Run("Test APIAPIService GetAPI", func(t *testing.T) {

		t.Skip("skip test") // remove to run test

		var id string

		resp, httpRes, err := apiClient.APIAPI.GetAPI(context.Background(), id).Execute()

		require.Nil(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 200, httpRes.StatusCode)

	})

	t.Run("Test APIAPIService ListAPI", func(t *testing.T) {

		t.Skip("skip test") // remove to run test

		resp, httpRes, err := apiClient.APIAPI.ListAPI(context.Background()).Execute()

		require.Nil(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 200, httpRes.StatusCode)

	})

}
