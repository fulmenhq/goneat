package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRunHome_NoFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("init", false, "")
	cmd.Flags().Bool("reset", false, "")

	err := runHome(cmd, []string{})
	if err != nil {
		t.Errorf("runHome with no flags should not return error: %v", err)
	}
}

func TestRunHome_InitFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("init", true, "")
	cmd.Flags().Bool("reset", false, "")

	// This will print messages but should not error
	err := runHome(cmd, []string{})
	if err != nil {
		t.Errorf("runHome with --init flag should not return error: %v", err)
	}
}

func TestRunHome_ResetFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("init", false, "")
	cmd.Flags().Bool("reset", true, "")

	// This will print messages but should not error
	err := runHome(cmd, []string{})
	if err != nil {
		t.Errorf("runHome with --reset flag should not return error: %v", err)
	}
}

func TestHomeCmd_Registration(t *testing.T) {
	// Test that homeCmd is properly configured
	if homeCmd.Use != "home" {
		t.Errorf("Expected homeCmd.Use to be 'home', got %q", homeCmd.Use)
	}
	if homeCmd.Short == "" {
		t.Error("homeCmd.Short should not be empty")
	}
	if homeCmd.Long == "" {
		t.Error("homeCmd.Long should not be empty")
	}

	// Test flags
	initFlag := homeCmd.Flag("init")
	if initFlag == nil {
		t.Error("homeCmd should have --init flag")
	} else if initFlag.Value.String() != "false" {
		t.Errorf("Expected --init flag default to be 'false', got %q", initFlag.Value.String())
	}

	resetFlag := homeCmd.Flag("reset")
	if resetFlag == nil {
		t.Error("homeCmd should have --reset flag")
	} else if resetFlag.Value.String() != "false" {
		t.Errorf("Expected --reset flag default to be 'false', got %q", resetFlag.Value.String())
	}
}

func TestHomeCmd_RunE(t *testing.T) {
	// Test that RunE is properly assigned
	if homeCmd.RunE == nil {
		t.Error("homeCmd.RunE should be assigned")
	}

	// Test that it's the runHome function
	cmd := &cobra.Command{}
	cmd.Flags().Bool("init", false, "")
	cmd.Flags().Bool("reset", false, "")

	// This should work without error
	err := homeCmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("homeCmd.RunE should work: %v", err)
	}
}
