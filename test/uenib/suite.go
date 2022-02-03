// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package uenib

import (
	"github.com/onosproject/helmit/pkg/input"
	"github.com/onosproject/helmit/pkg/test"
	"github.com/onosproject/onos-uenib/test/utils"
)

// TestSuite is the primary onos-uenib test suite
type TestSuite struct {
	test.Suite
	c *input.Context
}

// SetupTestSuite sets up the onos-e2t test suite
func (s *TestSuite) SetupTestSuite(c *input.Context) error {
	s.c = c
	sdran, err := utils.CreateSdranRelease(c)
	if err != nil {
		return err
	}

	registry := c.GetArg("registry").String("")

	return sdran.Set("global.image.registry", registry).Install(true)
}
