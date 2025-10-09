package publishers

// Test valid Docker publisher
validDockerPublisher: #PublisherConfig & {
	type:      "docker"
	registry:  "docker.io"
	namespace: "myorg"
}

// Test Docker publisher with credentials
dockerWithCredentials: #PublisherConfig & {
	type:      "docker"
	registry:  "ghcr.io"
	namespace: "github-org"
	credentials: {
		provider: "aws"
		name:     "docker-creds"
		region:   "us-east-1"
	}
}

// Test valid GitHub publisher
validGitHubPublisher: #PublisherConfig & {
	type:       "github"
	repository: "owner/repo"
}

// Test GitHub publisher with Vault credentials
githubWithVaultCredentials: #PublisherConfig & {
	type:       "github"
	repository: "myorg/myrepo"
	credentials: {
		provider: "vault"
		path:     "secret/data/github/token"
		key:      "token"
	}
}

// Test valid S3 publisher
validS3Publisher: #PublisherConfig & {
	type:   "s3"
	bucket: "my-artifacts-bucket"
}

// Test S3 publisher with region and credentials
s3WithRegionAndCreds: #PublisherConfig & {
	type:   "s3"
	bucket: "production-artifacts"
	region: "eu-west-1"
	credentials: {
		provider: "aws"
		name:     "s3-publish-creds"
		key:      "access_key"
	}
}
