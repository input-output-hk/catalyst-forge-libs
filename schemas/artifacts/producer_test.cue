package artifacts

// Test: Valid EarthlyProducer with just target
_validEarthlyProducerMinimal: #EarthlyProducer & {
	type:   "earthly"
	target: "+build"
}

// Test: Valid EarthlyProducer with target and artifact
_validEarthlyProducerWithArtifact: #EarthlyProducer & {
	type:     "earthly"
	target:   "+build"
	artifact: "+build/output"
}

// Test: ArtifactProducer union accepts EarthlyProducer
_unionAcceptsEarthly: #ArtifactProducer & _validEarthlyProducerMinimal

// Test: ArtifactProducer union accepts EarthlyProducer with artifact
_unionAcceptsEarthlyWithArtifact: #ArtifactProducer & _validEarthlyProducerWithArtifact

// Test: Multiple Earthly targets with different patterns
_earthlyWithComplexTarget: #EarthlyProducer & {
	type:   "earthly"
	target: "+build-and-test"
}

_earthlyWithArtifactPath: #EarthlyProducer & {
	type:     "earthly"
	target:   "+docker"
	artifact: "+docker/image.tar"
}
