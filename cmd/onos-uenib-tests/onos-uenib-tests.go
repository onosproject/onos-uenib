// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/onosproject/helmit/pkg/registry"
	"github.com/onosproject/helmit/pkg/test"
	"github.com/onosproject/onos-uenib/test/uenib"
)

func main() {
	registry.RegisterTestSuite("uenib", &uenib.TestSuite{})
	test.Main()
}
