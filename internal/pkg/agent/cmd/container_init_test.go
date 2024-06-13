// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build linux

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"kernel.org/pub/linux/libs/security/libcap/cap"
)

func Test_chownPaths(t *testing.T) {
	firstParentDir, err := os.MkdirTemp("", "test_chown")
	require.NoError(t, err)
	defer os.RemoveAll(firstParentDir)

	secondParentDir, err := os.MkdirTemp("", "test_chown")
	require.NoError(t, err)
	defer os.RemoveAll(secondParentDir)

	childDir := filepath.Join(secondParentDir, "child")
	err = os.MkdirAll(childDir, 0o777)
	require.NoError(t, err)

	childChildDir := filepath.Join(childDir, "child")
	err = os.MkdirAll(childDir, 0o777)
	require.NoError(t, err)

	pathsToChown := distinctPaths{}
	pathsToChown.addPath(childDir)
	pathsToChown.addPath(secondParentDir)
	pathsToChown.addPath(childChildDir)
	pathsToChown.addPath(firstParentDir)

	require.EqualValues(t, distinctPaths{firstParentDir: {}, secondParentDir: {}}, pathsToChown)

	err = pathsToChown.chown(os.Getuid(), os.Getpid())
	require.NoError(t, err)
}

func Test_getMissingBoundingCapsText(t *testing.T) {
	tc := []struct {
		name      string
		fileCaps  []cap.Value
		boundCaps []cap.Value
		capsText  string
	}{
		{
			name:      "no missing caps",
			fileCaps:  []cap.Value{cap.CHOWN, cap.SETPCAP},
			boundCaps: []cap.Value{cap.CHOWN, cap.SETPCAP},
			capsText:  "",
		},
		{
			name:      "missing caps",
			fileCaps:  []cap.Value{cap.CHOWN, cap.SETPCAP},
			boundCaps: []cap.Value{cap.CHOWN, cap.SETPCAP, cap.DAC_OVERRIDE},
			capsText:  "cap_chown,cap_dac_override,cap_setpcap=eip",
		},
	}

	defer func() {
		capBound = cap.GetBound
		capGetFile = cap.GetFile
	}()

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			capBound = func(val cap.Value) (bool, error) {
				for _, boundCap := range tt.boundCaps {
					if boundCap == val {
						return true, nil
					}
				}
				return false, nil
			}
			capGetFile = func(path string) (*cap.Set, error) {
				set := cap.NewSet()

				if err := set.SetFlag(cap.Effective, true, tt.fileCaps...); err != nil {
					return nil, err
				}

				return set, nil
			}

			capsText, err := getMissingBoundingCapsText("non_existent")
			require.NoError(t, err)
			require.Equal(t, tt.capsText, capsText)
		})
	}
}

func Test_getAmbientCapabilitiesFromEffectiveSet(t *testing.T) {
	defer func() {
		capProc = cap.GetProc
	}()

	tc := []struct {
		name         string
		procCaps     []cap.Value
		expectedCaps []cap.Value
	}{
		{
			name:         "no ambient caps",
			procCaps:     []cap.Value{cap.CHOWN, cap.SETPCAP, cap.SETFCAP},
			expectedCaps: []cap.Value(nil),
		},
		{
			name:         "no ambient caps",
			procCaps:     []cap.Value{cap.CHOWN, cap.SETPCAP, cap.SETFCAP, cap.BPF},
			expectedCaps: []cap.Value{cap.BPF},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			capProc = func() *cap.Set {
				set := cap.NewSet()

				if err := set.SetFlag(cap.Effective, true, tt.procCaps...); err != nil {
					return nil
				}

				return set
			}

			capsText, err := getAmbientCapabilitiesFromEffectiveSet()
			require.NoError(t, err)
			require.Equal(t, tt.expectedCaps, capsText)
		})
	}
}
