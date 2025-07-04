// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || windows

package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/runfinch/finch/pkg/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewStopVMCommand(t *testing.T) {
	t.Parallel()

	cmd := newStopVMCommand(nil, nil, nil)
	assert.Equal(t, cmd.Name(), "stop")
}

func TestStopVMAction_runAdapter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mockSvc func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager)
		args    []string
		wantErr error
	}{
		{
			name: "should stop the instance",
			args: []string{},
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Running"), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "Running")

				dm.EXPECT().DetachUserDataDisk().Return(nil)

				command := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("stop", limaInstanceName).Return(command)
				command.EXPECT().CombinedOutput()
				logger.EXPECT().Info(gomock.Any()).AnyTimes()
			},
			wantErr: nil,
		},
		{
			name: "should force stop the instance",
			args: []string{
				"--force",
			},
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager) {
				command := mocks.NewCommand(ctrl)
				dm.EXPECT().DetachUserDataDisk().Return(nil)
				creator.EXPECT().CreateWithoutStdio("stop", "--force", limaInstanceName).Return(command)
				command.EXPECT().CombinedOutput()
				logger.EXPECT().Info(gomock.Any()).AnyTimes()
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			dm := mocks.NewUserDataDiskManager(ctrl)
			logger := mocks.NewLogger(ctrl)
			ncc := mocks.NewNerdctlCmdCreator(ctrl)
			tc.mockSvc(logger, ncc, ctrl, dm)

			cmd := newStopVMCommand(ncc, dm, logger)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestStopVMAction_run(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		wantErr error
		mockSvc func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager)
		force   bool
	}{
		{
			name:    "should stop the instance",
			wantErr: nil,
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Running"), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "Running")
				dm.EXPECT().DetachUserDataDisk().Return(nil)

				command := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("stop", limaInstanceName).Return(command)
				command.EXPECT().CombinedOutput()
				logger.EXPECT().Info("Stopping existing Finch virtual machine...")
				logger.EXPECT().Info("Finch virtual machine stopped successfully")
			},
			force: false,
		},
		{
			name:    "stopped VM",
			wantErr: fmt.Errorf("the instance %q is already stopped", limaInstanceName),
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, _ *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Stopped"), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "Stopped")
			},
			force: false,
		},
		{
			name:    "nonexistent VM",
			wantErr: fmt.Errorf("the instance %q does not exist", limaInstanceName),
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, _ *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte(""), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "")
			},
			force: false,
		},
		{
			name:    "unknown VM status",
			wantErr: errors.New("unrecognized system status"),
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, _ *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Broken"), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "Broken")
			},
			force: false,
		},
		{
			name:    "status command returns an error",
			wantErr: errors.New("get status error"),
			mockSvc: func(_ *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, _ *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Broken"), errors.New("get status error"))
			},
			force: false,
		},
		{
			name:    "should print error if virtual machine failed to stop",
			wantErr: errors.New("error"),
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager) {
				getVMStatusC := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("ls", "-f", "{{.Status}}", limaInstanceName).Return(getVMStatusC)
				getVMStatusC.EXPECT().Output().Return([]byte("Running"), nil)
				logger.EXPECT().Debugf("Status of virtual machine: %s", "Running")
				dm.EXPECT().DetachUserDataDisk().Return(nil)

				logs := []byte("stdout + stderr")
				command := mocks.NewCommand(ctrl)
				command.EXPECT().CombinedOutput().Return(logs, errors.New("error"))
				creator.EXPECT().CreateWithoutStdio("stop", limaInstanceName).Return(command)
				logger.EXPECT().Info("Stopping existing Finch virtual machine...")
				logger.EXPECT().Errorf("Finch virtual machine failed to stop, debug logs:\n%s", logs)
			},
			force: false,
		},
		{
			name:    "should force stop virtual machine",
			wantErr: nil,
			mockSvc: func(logger *mocks.Logger, creator *mocks.NerdctlCmdCreator, ctrl *gomock.Controller, dm *mocks.UserDataDiskManager) {
				command := mocks.NewCommand(ctrl)
				creator.EXPECT().CreateWithoutStdio("stop", "--force", limaInstanceName).Return(command)
				command.EXPECT().CombinedOutput()
				dm.EXPECT().DetachUserDataDisk().Return(nil)
				logger.EXPECT().Info("Forcibly stopping Finch virtual machine...")
				logger.EXPECT().Info("Finch virtual machine stopped successfully")
			},
			force: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			dm := mocks.NewUserDataDiskManager(ctrl)
			logger := mocks.NewLogger(ctrl)
			ncc := mocks.NewNerdctlCmdCreator(ctrl)

			tc.mockSvc(logger, ncc, ctrl, dm)
			err := newStopVMAction(ncc, dm, logger).run(tc.force)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
