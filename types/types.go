package types

type RestoreParams struct {
	AssetBucketList       []string `json:"assetBucketList"`
	ManifestKey           string   `json:"manifestKey"`
	ManifestBucket        string   `json:"manifestBucket"`
	ManifestLocalPath     string   `json:"manifestLocalPath"`
	RoleArn               string   `json:"roleArn"`
	AWS_ACCESS_KEY_ID     string   `json:"aws_access_key_id"`
	AWS_SECRET_ACCESS_KEY string   `json:"aws_secret_access_key"`
	AWS_DEFAULT_REGION    string   `json:"aws_default_region"`
	ProjectId             int      `json:"projectId"`
	User                  string   `json:"user"`
	RestorePath           string   `json:"restorePath"`
}

type RequestBody struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
	User string `json:"user"`
}
