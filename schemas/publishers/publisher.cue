package publishers

// PublisherConfig is a discriminated union of all publisher types.
// Each publisher is distinguished by its type! field.
// MVP supports: docker, github, s3.
#PublisherConfig: #DockerPublisher | #GitHubPublisher | #S3Publisher
