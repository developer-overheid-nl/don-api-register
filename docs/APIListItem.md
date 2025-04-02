# APIListItem

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**ServiceName** | Pointer to **string** |  | [optional] 
**Description** | Pointer to **string** |  | [optional] 
**Organization** | Pointer to [**APIListItemOrganization**](APIListItemOrganization.md) |  | [optional] 
**ApiType** | Pointer to **string** |  | [optional] 
**ApiAuthentication** | Pointer to **string** |  | [optional] 
**Environments** | Pointer to [**[]APIListItemEnvironmentsInner**](APIListItemEnvironmentsInner.md) |  | [optional] 
**Contact** | Pointer to [**APIListItemContact**](APIListItemContact.md) |  | [optional] 
**IsReferenceImplementation** | Pointer to **bool** |  | [optional] 
**TermsOfUse** | Pointer to [**APIListItemTermsOfUse**](APIListItemTermsOfUse.md) |  | [optional] 

## Methods

### NewAPIListItem

`func NewAPIListItem() *APIListItem`

NewAPIListItem instantiates a new APIListItem object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAPIListItemWithDefaults

`func NewAPIListItemWithDefaults() *APIListItem`

NewAPIListItemWithDefaults instantiates a new APIListItem object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *APIListItem) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *APIListItem) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *APIListItem) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *APIListItem) HasId() bool`

HasId returns a boolean if a field has been set.

### GetServiceName

`func (o *APIListItem) GetServiceName() string`

GetServiceName returns the ServiceName field if non-nil, zero value otherwise.

### GetServiceNameOk

`func (o *APIListItem) GetServiceNameOk() (*string, bool)`

GetServiceNameOk returns a tuple with the ServiceName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetServiceName

`func (o *APIListItem) SetServiceName(v string)`

SetServiceName sets ServiceName field to given value.

### HasServiceName

`func (o *APIListItem) HasServiceName() bool`

HasServiceName returns a boolean if a field has been set.

### GetDescription

`func (o *APIListItem) GetDescription() string`

GetDescription returns the Description field if non-nil, zero value otherwise.

### GetDescriptionOk

`func (o *APIListItem) GetDescriptionOk() (*string, bool)`

GetDescriptionOk returns a tuple with the Description field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDescription

`func (o *APIListItem) SetDescription(v string)`

SetDescription sets Description field to given value.

### HasDescription

`func (o *APIListItem) HasDescription() bool`

HasDescription returns a boolean if a field has been set.

### GetOrganization

`func (o *APIListItem) GetOrganization() APIListItemOrganization`

GetOrganization returns the Organization field if non-nil, zero value otherwise.

### GetOrganizationOk

`func (o *APIListItem) GetOrganizationOk() (*APIListItemOrganization, bool)`

GetOrganizationOk returns a tuple with the Organization field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOrganization

`func (o *APIListItem) SetOrganization(v APIListItemOrganization)`

SetOrganization sets Organization field to given value.

### HasOrganization

`func (o *APIListItem) HasOrganization() bool`

HasOrganization returns a boolean if a field has been set.

### GetApiType

`func (o *APIListItem) GetApiType() string`

GetApiType returns the ApiType field if non-nil, zero value otherwise.

### GetApiTypeOk

`func (o *APIListItem) GetApiTypeOk() (*string, bool)`

GetApiTypeOk returns a tuple with the ApiType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetApiType

`func (o *APIListItem) SetApiType(v string)`

SetApiType sets ApiType field to given value.

### HasApiType

`func (o *APIListItem) HasApiType() bool`

HasApiType returns a boolean if a field has been set.

### GetApiAuthentication

`func (o *APIListItem) GetApiAuthentication() string`

GetApiAuthentication returns the ApiAuthentication field if non-nil, zero value otherwise.

### GetApiAuthenticationOk

`func (o *APIListItem) GetApiAuthenticationOk() (*string, bool)`

GetApiAuthenticationOk returns a tuple with the ApiAuthentication field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetApiAuthentication

`func (o *APIListItem) SetApiAuthentication(v string)`

SetApiAuthentication sets ApiAuthentication field to given value.

### HasApiAuthentication

`func (o *APIListItem) HasApiAuthentication() bool`

HasApiAuthentication returns a boolean if a field has been set.

### GetEnvironments

`func (o *APIListItem) GetEnvironments() []APIListItemEnvironmentsInner`

GetEnvironments returns the Environments field if non-nil, zero value otherwise.

### GetEnvironmentsOk

`func (o *APIListItem) GetEnvironmentsOk() (*[]APIListItemEnvironmentsInner, bool)`

GetEnvironmentsOk returns a tuple with the Environments field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvironments

`func (o *APIListItem) SetEnvironments(v []APIListItemEnvironmentsInner)`

SetEnvironments sets Environments field to given value.

### HasEnvironments

`func (o *APIListItem) HasEnvironments() bool`

HasEnvironments returns a boolean if a field has been set.

### GetContact

`func (o *APIListItem) GetContact() APIListItemContact`

GetContact returns the Contact field if non-nil, zero value otherwise.

### GetContactOk

`func (o *APIListItem) GetContactOk() (*APIListItemContact, bool)`

GetContactOk returns a tuple with the Contact field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetContact

`func (o *APIListItem) SetContact(v APIListItemContact)`

SetContact sets Contact field to given value.

### HasContact

`func (o *APIListItem) HasContact() bool`

HasContact returns a boolean if a field has been set.

### GetIsReferenceImplementation

`func (o *APIListItem) GetIsReferenceImplementation() bool`

GetIsReferenceImplementation returns the IsReferenceImplementation field if non-nil, zero value otherwise.

### GetIsReferenceImplementationOk

`func (o *APIListItem) GetIsReferenceImplementationOk() (*bool, bool)`

GetIsReferenceImplementationOk returns a tuple with the IsReferenceImplementation field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIsReferenceImplementation

`func (o *APIListItem) SetIsReferenceImplementation(v bool)`

SetIsReferenceImplementation sets IsReferenceImplementation field to given value.

### HasIsReferenceImplementation

`func (o *APIListItem) HasIsReferenceImplementation() bool`

HasIsReferenceImplementation returns a boolean if a field has been set.

### GetTermsOfUse

`func (o *APIListItem) GetTermsOfUse() APIListItemTermsOfUse`

GetTermsOfUse returns the TermsOfUse field if non-nil, zero value otherwise.

### GetTermsOfUseOk

`func (o *APIListItem) GetTermsOfUseOk() (*APIListItemTermsOfUse, bool)`

GetTermsOfUseOk returns a tuple with the TermsOfUse field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTermsOfUse

`func (o *APIListItem) SetTermsOfUse(v APIListItemTermsOfUse)`

SetTermsOfUse sets TermsOfUse field to given value.

### HasTermsOfUse

`func (o *APIListItem) HasTermsOfUse() bool`

HasTermsOfUse returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


