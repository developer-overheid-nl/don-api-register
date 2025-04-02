# RepositoryListItem

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Source** | Pointer to **string** |  | [optional] 
**OwnerName** | Pointer to **string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Description** | Pointer to **string** |  | [optional] 
**LastChange** | Pointer to **time.Time** |  | [optional] 
**Url** | Pointer to **string** |  | [optional] 
**AvatarUrl** | Pointer to **string** |  | [optional] 
**Stars** | Pointer to **int32** |  | [optional] 
**ForkCount** | Pointer to **int32** |  | [optional] 
**IssueOpenCount** | Pointer to **int32** |  | [optional] 
**MergeRequestOpenCount** | Pointer to **int32** |  | [optional] 
**Archived** | Pointer to **bool** |  | [optional] 
**ProgrammingLanguages** | Pointer to **[]string** |  | [optional] 

## Methods

### NewRepositoryListItem

`func NewRepositoryListItem() *RepositoryListItem`

NewRepositoryListItem instantiates a new RepositoryListItem object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRepositoryListItemWithDefaults

`func NewRepositoryListItemWithDefaults() *RepositoryListItem`

NewRepositoryListItemWithDefaults instantiates a new RepositoryListItem object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetSource

`func (o *RepositoryListItem) GetSource() string`

GetSource returns the Source field if non-nil, zero value otherwise.

### GetSourceOk

`func (o *RepositoryListItem) GetSourceOk() (*string, bool)`

GetSourceOk returns a tuple with the Source field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSource

`func (o *RepositoryListItem) SetSource(v string)`

SetSource sets Source field to given value.

### HasSource

`func (o *RepositoryListItem) HasSource() bool`

HasSource returns a boolean if a field has been set.

### GetOwnerName

`func (o *RepositoryListItem) GetOwnerName() string`

GetOwnerName returns the OwnerName field if non-nil, zero value otherwise.

### GetOwnerNameOk

`func (o *RepositoryListItem) GetOwnerNameOk() (*string, bool)`

GetOwnerNameOk returns a tuple with the OwnerName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOwnerName

`func (o *RepositoryListItem) SetOwnerName(v string)`

SetOwnerName sets OwnerName field to given value.

### HasOwnerName

`func (o *RepositoryListItem) HasOwnerName() bool`

HasOwnerName returns a boolean if a field has been set.

### GetName

`func (o *RepositoryListItem) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *RepositoryListItem) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *RepositoryListItem) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *RepositoryListItem) HasName() bool`

HasName returns a boolean if a field has been set.

### GetDescription

`func (o *RepositoryListItem) GetDescription() string`

GetDescription returns the Description field if non-nil, zero value otherwise.

### GetDescriptionOk

`func (o *RepositoryListItem) GetDescriptionOk() (*string, bool)`

GetDescriptionOk returns a tuple with the Description field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDescription

`func (o *RepositoryListItem) SetDescription(v string)`

SetDescription sets Description field to given value.

### HasDescription

`func (o *RepositoryListItem) HasDescription() bool`

HasDescription returns a boolean if a field has been set.

### GetLastChange

`func (o *RepositoryListItem) GetLastChange() time.Time`

GetLastChange returns the LastChange field if non-nil, zero value otherwise.

### GetLastChangeOk

`func (o *RepositoryListItem) GetLastChangeOk() (*time.Time, bool)`

GetLastChangeOk returns a tuple with the LastChange field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastChange

`func (o *RepositoryListItem) SetLastChange(v time.Time)`

SetLastChange sets LastChange field to given value.

### HasLastChange

`func (o *RepositoryListItem) HasLastChange() bool`

HasLastChange returns a boolean if a field has been set.

### GetUrl

`func (o *RepositoryListItem) GetUrl() string`

GetUrl returns the Url field if non-nil, zero value otherwise.

### GetUrlOk

`func (o *RepositoryListItem) GetUrlOk() (*string, bool)`

GetUrlOk returns a tuple with the Url field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUrl

`func (o *RepositoryListItem) SetUrl(v string)`

SetUrl sets Url field to given value.

### HasUrl

`func (o *RepositoryListItem) HasUrl() bool`

HasUrl returns a boolean if a field has been set.

### GetAvatarUrl

`func (o *RepositoryListItem) GetAvatarUrl() string`

GetAvatarUrl returns the AvatarUrl field if non-nil, zero value otherwise.

### GetAvatarUrlOk

`func (o *RepositoryListItem) GetAvatarUrlOk() (*string, bool)`

GetAvatarUrlOk returns a tuple with the AvatarUrl field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAvatarUrl

`func (o *RepositoryListItem) SetAvatarUrl(v string)`

SetAvatarUrl sets AvatarUrl field to given value.

### HasAvatarUrl

`func (o *RepositoryListItem) HasAvatarUrl() bool`

HasAvatarUrl returns a boolean if a field has been set.

### GetStars

`func (o *RepositoryListItem) GetStars() int32`

GetStars returns the Stars field if non-nil, zero value otherwise.

### GetStarsOk

`func (o *RepositoryListItem) GetStarsOk() (*int32, bool)`

GetStarsOk returns a tuple with the Stars field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStars

`func (o *RepositoryListItem) SetStars(v int32)`

SetStars sets Stars field to given value.

### HasStars

`func (o *RepositoryListItem) HasStars() bool`

HasStars returns a boolean if a field has been set.

### GetForkCount

`func (o *RepositoryListItem) GetForkCount() int32`

GetForkCount returns the ForkCount field if non-nil, zero value otherwise.

### GetForkCountOk

`func (o *RepositoryListItem) GetForkCountOk() (*int32, bool)`

GetForkCountOk returns a tuple with the ForkCount field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetForkCount

`func (o *RepositoryListItem) SetForkCount(v int32)`

SetForkCount sets ForkCount field to given value.

### HasForkCount

`func (o *RepositoryListItem) HasForkCount() bool`

HasForkCount returns a boolean if a field has been set.

### GetIssueOpenCount

`func (o *RepositoryListItem) GetIssueOpenCount() int32`

GetIssueOpenCount returns the IssueOpenCount field if non-nil, zero value otherwise.

### GetIssueOpenCountOk

`func (o *RepositoryListItem) GetIssueOpenCountOk() (*int32, bool)`

GetIssueOpenCountOk returns a tuple with the IssueOpenCount field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIssueOpenCount

`func (o *RepositoryListItem) SetIssueOpenCount(v int32)`

SetIssueOpenCount sets IssueOpenCount field to given value.

### HasIssueOpenCount

`func (o *RepositoryListItem) HasIssueOpenCount() bool`

HasIssueOpenCount returns a boolean if a field has been set.

### GetMergeRequestOpenCount

`func (o *RepositoryListItem) GetMergeRequestOpenCount() int32`

GetMergeRequestOpenCount returns the MergeRequestOpenCount field if non-nil, zero value otherwise.

### GetMergeRequestOpenCountOk

`func (o *RepositoryListItem) GetMergeRequestOpenCountOk() (*int32, bool)`

GetMergeRequestOpenCountOk returns a tuple with the MergeRequestOpenCount field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMergeRequestOpenCount

`func (o *RepositoryListItem) SetMergeRequestOpenCount(v int32)`

SetMergeRequestOpenCount sets MergeRequestOpenCount field to given value.

### HasMergeRequestOpenCount

`func (o *RepositoryListItem) HasMergeRequestOpenCount() bool`

HasMergeRequestOpenCount returns a boolean if a field has been set.

### GetArchived

`func (o *RepositoryListItem) GetArchived() bool`

GetArchived returns the Archived field if non-nil, zero value otherwise.

### GetArchivedOk

`func (o *RepositoryListItem) GetArchivedOk() (*bool, bool)`

GetArchivedOk returns a tuple with the Archived field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetArchived

`func (o *RepositoryListItem) SetArchived(v bool)`

SetArchived sets Archived field to given value.

### HasArchived

`func (o *RepositoryListItem) HasArchived() bool`

HasArchived returns a boolean if a field has been set.

### GetProgrammingLanguages

`func (o *RepositoryListItem) GetProgrammingLanguages() []string`

GetProgrammingLanguages returns the ProgrammingLanguages field if non-nil, zero value otherwise.

### GetProgrammingLanguagesOk

`func (o *RepositoryListItem) GetProgrammingLanguagesOk() (*[]string, bool)`

GetProgrammingLanguagesOk returns a tuple with the ProgrammingLanguages field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProgrammingLanguages

`func (o *RepositoryListItem) SetProgrammingLanguages(v []string)`

SetProgrammingLanguages sets ProgrammingLanguages field to given value.

### HasProgrammingLanguages

`func (o *RepositoryListItem) HasProgrammingLanguages() bool`

HasProgrammingLanguages returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


