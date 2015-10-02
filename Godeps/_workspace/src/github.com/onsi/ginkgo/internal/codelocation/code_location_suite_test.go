package codelocation_test

import (
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/gomega"

	"testing"
)

func TestCodelocation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CodeLocation Suite")
}
