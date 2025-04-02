# APIListItemOrganization

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Ooid** | Pointer to **int32** | Organisaties overheid ID | [optional] 
**Name** | Pointer to **string** |  | [optional] 

## Methods

### NewAPIListItemOrganization

`func NewAPIListItemOrganization() *APIListItemOrganization`

NewAPIListItemOrganization instantiates a new APIListItemOrganization object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAPIListItemOrganizationWithDefaults

`func NewAPIListItemOrganizationWithDefaults() *APIListItemOrganization`

NewAPIListItemOrganizationWithDefaults instantiates a new APIListItemOrganization object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetOoid

`func (o *APIListItemOrganization) GetOoid() int32`

GetOoid returns the Ooid field if non-nil, zero value otherwise.

### GetOoidOk

`func (o *APIListItemOrganization) GetOoidOk() (*int32, bool)`

GetOoidOk returns a tuple with the Ooid field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOoid

`func (o *APIListItemOrganization) SetOoid(v int32)`

SetOoid sets Ooid field to given value.

### HasOoid

`func (o *APIListItemOrganization) HasOoid() bool`

HasOoid returns a boolean if a field has been set.

### GetName

`func (o *APIListItemOrganization) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *APIListItemOrganization) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *APIListItemOrganization) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *APIListItemOrganization) HasName() bool`

HasName returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


