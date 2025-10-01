package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCreateFstabLines(t *testing.T) {
	t.Run("single path", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/var/lib/bindicate/protected",
			Paths:  []string{"/etc/hostname"},
		}

		result := b.createFstabLines()
		expected := `/var/lib/bindicate/protected/etc/hostname /etc/hostname bind  0 0
`

		if result != expected {
			t.Errorf("Expected fstab line:\n%q\nGot:\n%q", expected, result)
		}
	})

	t.Run("multiple paths", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/test/prefix",
			Paths:  []string{"/etc/hostname", "/etc/hosts", "/etc/fstab"},
		}

		result := b.createFstabLines()
		expected := `/test/prefix/etc/hostname /etc/hostname bind  0 0
/test/prefix/etc/hosts /etc/hosts bind  0 0
/test/prefix/etc/fstab /etc/fstab bind  0 0
`

		if result != expected {
			t.Errorf("Expected fstab lines:\n%q\nGot:\n%q", expected, result)
		}
	})

	t.Run("empty paths", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/test/prefix",
			Paths:  []string{},
		}

		result := b.createFstabLines()

		if result != "" {
			t.Errorf("Expected empty result for empty paths, got: %q", result)
		}
	})
}

func TestSyncBindMountsInFstab(t *testing.T) {
	t.Run("replace existing bindicate section", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/test",
			Paths:  []string{"/etc/test"},
		}

		originalFstab := `/dev/sda1 / ext4 defaults 0 1
# BINDICATE START
/old/path /old/target bind defaults 0 0
# BINDICATE END
# More entries
/dev/sda2 /home ext4 defaults 0 2`

		result := b.SyncBindMountsInFstab(originalFstab)
		expected := `/dev/sda1 / ext4 defaults 0 1
# More entries
/dev/sda2 /home ext4 defaults 0 2
# BINDICATE START
/test/etc/test /etc/test bind  0 0
# BINDICATE END`

		if result != expected {
			t.Errorf("Expected fstab:\n%q\nGot:\n%q", expected, result)
		}
	})

	t.Run("add bindicate section to fstab without existing section", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/test",
			Paths:  []string{"/etc/hostname"},
		}

		originalFstab := `# System fstab
/dev/sda1 / ext4 defaults 0 1
/dev/sda2 /home ext4 defaults 0 2`

		result := b.SyncBindMountsInFstab(originalFstab)
		expected := `# System fstab
/dev/sda1 / ext4 defaults 0 1
/dev/sda2 /home ext4 defaults 0 2
# BINDICATE START
/test/etc/hostname /etc/hostname bind  0 0
# BINDICATE END`

		if result != expected {
			t.Errorf("Expected fstab:\n%q\nGot:\n%q", expected, result)
		}
	})

	t.Run("handle empty fstab", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/test",
			Paths:  []string{"/etc/test"},
		}

		result := b.SyncBindMountsInFstab("")
		expected := `# BINDICATE START
/test/etc/test /etc/test bind  0 0
# BINDICATE END`

		if result != expected {
			t.Errorf("Expected fstab:\n%q\nGot:\n%q", expected, result)
		}
	})

	t.Run("multiple paths in fstab", func(t *testing.T) {
		b := Bindicate{
			Prefix: "/protected",
			Paths:  []string{"/etc/hostname", "/etc/hosts"},
		}

		originalFstab := `/dev/root / ext4 defaults 0 1`

		result := b.SyncBindMountsInFstab(originalFstab)
		expected := `/dev/root / ext4 defaults 0 1
# BINDICATE START
/protected/etc/hostname /etc/hostname bind  0 0
/protected/etc/hosts /etc/hosts bind  0 0
# BINDICATE END`

		if result != expected {
			t.Errorf("Expected fstab:\n%q\nGot:\n%q", expected, result)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid config file", func(t *testing.T) {
		configContent := `{
	"Prefix": "/var/lib/bindicate/protected",
	"Paths": ["/etc/hostname", "/etc/hosts"]
}`

		tmpfile, err := os.CreateTemp("", "bindicate-test-*.json")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		if _, err := tmpfile.Write([]byte(configContent)); err != nil {
			t.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpfile.Name())

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if result.Prefix != "/var/lib/bindicate/protected" {
			t.Errorf("Expected prefix '/var/lib/bindicate/protected', got: %s", result.Prefix)
		}
		if len(result.Paths) != 2 {
			t.Errorf("Expected 2 paths, got: %d", len(result.Paths))
		}
		if result.Paths[0] != "/etc/hostname" || result.Paths[1] != "/etc/hosts" {
			t.Errorf("Expected paths [/etc/hostname, /etc/hosts], got: %v", result.Paths)
		}
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/config.json")

		if err == nil {
			t.Errorf("Expected error for nonexistent file, got nil")
		}
	})

	t.Run("invalid JSON config", func(t *testing.T) {
		invalidJSON := `{
	"Prefix": "/test",
	"Paths": ["/etc/test"
}`

		tmpfile, err := os.CreateTemp("", "bindicate-test-invalid-*.json")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		if _, err := tmpfile.Write([]byte(invalidJSON)); err != nil {
			t.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			t.Fatal(err)
		}

		_, err = LoadConfig(tmpfile.Name())

		if err == nil {
			t.Errorf("Expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty config file", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "bindicate-test-empty-*.json")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		if err := tmpfile.Close(); err != nil {
			t.Fatal(err)
		}

		_, err = LoadConfig(tmpfile.Name())

		if err == nil {
			t.Errorf("Expected error for empty config file, got nil")
		}
	})
}

// Helper function to create temporary files for testing
func createTempConfigFile(t *testing.T, content string) string {
	tmpfile, err := os.CreateTemp("", "bindicate-test-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		os.Remove(tmpfile.Name())
		t.Fatal(err)
	}

	return tmpfile.Name()
}

func TestCopyFilePreservePermissions(t *testing.T) {
	t.Run("copy file with preserved permissions", func(t *testing.T) {
		// Create a temporary source file with specific permissions
		srcFile, err := os.CreateTemp("", "bindicate-test-src-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(srcFile.Name())

		testContent := "test file content"
		if _, err := srcFile.WriteString(testContent); err != nil {
			t.Fatal(err)
		}

		// Set specific permissions on source file
		testMode := os.FileMode(0641)
		if err := srcFile.Chmod(testMode); err != nil {
			t.Fatal(err)
		}
		srcFile.Close()

		// destination path
		dstFile := filepath.Join(os.TempDir(), "bindicate-test-dst-"+filepath.Base(srcFile.Name()))
		defer os.Remove(dstFile)

		err = copyFilePreservePermissions(srcFile.Name(), dstFile)
		if err != nil {
			t.Fatalf("copyFilePreservePermissions failed: %v", err)
		}

		// check exists
		if _, err := os.Stat(dstFile); os.IsNotExist(err) {
			t.Errorf("Destination file was not created")
		}

		// check content
		dstContent, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(dstContent) != testContent {
			t.Errorf("File content not copied correctly. Expected: %s, Got: %s", testContent, string(dstContent))
		}

		// check permissions
		dstInfo, err := os.Stat(dstFile)
		if err != nil {
			t.Fatal(err)
		}
		if dstInfo.Mode() != testMode {
			t.Errorf("File permissions not preserved. Expected: %v, Got: %v", testMode, dstInfo.Mode())
		}
	})

	t.Run("copy nonexistent source file", func(t *testing.T) {
		err := copyFilePreservePermissions("/nonexistent/source", "/tmp/dest")
		if err == nil {
			t.Errorf("Expected error when copying nonexistent source file, got nil")
		}
	})

	t.Run("copy to invalid destination", func(t *testing.T) {
		// Create a temporary source file
		srcFile, err := os.CreateTemp("", "bindicate-test-src-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(srcFile.Name())
		srcFile.Close()

		// Try to copy to an invalid destination (directory that doesn't exist)
		err = copyFilePreservePermissions(srcFile.Name(), "/nonexistent/directory/file")
		if err == nil {
			t.Errorf("Expected error when copying to invalid destination, got nil")
		}
	})
}

func TestBindMount(t *testing.T) {
	// Check if we're running as root or have CAP_SYS_ADMIN capability
	if os.Geteuid() != 0 {
		t.Skip("Skipping bind mount tests: requires root privileges or CAP_SYS_ADMIN capability")
	}

	t.Run("successful bind mount", func(t *testing.T) {
		// Create temporary source and destination directories
		srcDir, err := os.MkdirTemp("", "bindicate-test-src-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(srcDir)

		dstDir, err := os.MkdirTemp("", "bindicate-test-dst-*")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			// Unmount before cleanup
			syscall.Unmount(dstDir, 0)
			os.RemoveAll(dstDir)
		}()

		// Create a test file in source directory
		testFile := filepath.Join(srcDir, "testfile")
		testContent := "bind mount test content"
		if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Perform bind mount
		err = BindMount(srcDir, dstDir)
		if err != nil {
			t.Fatalf("BindMount failed: %v", err)
		}

		// Verify bind mount worked by checking if file is accessible in destination
		mountedFile := filepath.Join(dstDir, "testfile")
		content, err := os.ReadFile(mountedFile)
		if err != nil {
			t.Errorf("Could not read file from bind mount: %v", err)
		} else if string(content) != testContent {
			t.Errorf("Bind mount content mismatch. Expected: %s, Got: %s", testContent, string(content))
		}

		// Clean up: unmount
		if err := syscall.Unmount(dstDir, 0); err != nil {
			t.Logf("Warning: failed to unmount %s: %v", dstDir, err)
		}
	})

	t.Run("bind mount nonexistent source", func(t *testing.T) {
		dstDir, err := os.MkdirTemp("", "bindicate-test-dst-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dstDir)

		err = BindMount("/nonexistent/source", dstDir)
		if err == nil {
			t.Errorf("Expected error when bind mounting nonexistent source, got nil")
		}
	})

	t.Run("bind mount to nonexistent destination", func(t *testing.T) {
		srcDir, err := os.MkdirTemp("", "bindicate-test-src-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(srcDir)

		err = BindMount(srcDir, "/nonexistent/destination")
		if err == nil {
			t.Errorf("Expected error when bind mounting to nonexistent destination, got nil")
		}
	})
}

// Test helper function to check if we have mount privileges
func TestBindMountPermissions(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Skipping bind mount permission tests: requires root privileges or CAP_SYS_ADMIN capability")
	}

	// Try a simple mount operation to test permissions
	tmpDir1, err := os.MkdirTemp("", "bindicate-perm-test1-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "bindicate-perm-test2-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir2)

	err = BindMount(tmpDir1, tmpDir2)
	if err != nil {
		t.Errorf("Bind mount failed despite running as root: %v", err)
	} else {
		t.Log("Successfully performed bind mount as root")
		syscall.Unmount(tmpDir2, 0) // Clean up
	}
}
