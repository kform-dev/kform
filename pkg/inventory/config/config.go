package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/henderiw/logger/log"
	invv1alpha1 "github.com/kform-dev/kform/apis/inv/v1alpha1"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/inventory/config/configmap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/yaml"
)

const (
	// Kform uses a fixed file to store the inventory reference
	// <PKG-DIR>/.kform/kform-inventory.yaml
	kformInventoryManifestFilename   = "kform-inventory.yaml"
	kformInventoryManifestFileSubDir = ".kform"
	// Kform uses a fixed namespace to store inventory data
	kformInventoryNamespace = "kform-system"
	// Must begin and end with an alphanumeric character ([a-z0-9A-Z])
	// with dashes (-), underscores (_), dots (.), and alphanumerics
	// between.
	inventoryIDRegexp = `^[a-zA-Z0-9][a-zA-Z0-9\-\_\.]+[a-zA-Z0-9]$`
)

// InitOptions contains the fields necessary to generate a
// inventory object template ConfigMap.
type Config struct {
	ioStreams genericclioptions.IOStreams
	// Template string; must be a valid k8s resource.
	Template string
	// Package directory argument; must be valid directory.
	Dir string
	// Namespace for inventory object; Kform is fixed to kform-system.
	Namespace string
	// Inventory object label value; must be a valid k8s label value.
	InventoryID string
}

func New(ioStreams genericclioptions.IOStreams) *Config {
	return &Config{
		ioStreams: ioStreams,
		Template:  configmap.ConfigMapTemplate, // fixed <PKG-DIR>/.kform/kform-inventory.yaml
		Namespace: kformInventoryNamespace,     // fixed namespace for kform
	}
}

func (r *Config) Complete(ctx context.Context, path string) error {
	log := log.FromContext(ctx)

	dir, err := fsys.NormalizeDir(path)
	if err != nil {
		return err
	}
	r.Dir = dir
	log.Debug("package directory", "dir", r.Dir)

	// Set the default inventory label if one does not exist.
	if len(r.InventoryID) == 0 {
		inventoryID, err := r.defaultInventoryID()
		if err != nil {
			return err
		}
		r.InventoryID = inventoryID
	}
	if !validateInventoryID(r.InventoryID) {
		return fmt.Errorf("invalid group name: %s", r.InventoryID)
	}
	// Output the calculated namespace used for inventory object.
	fmt.Fprintf(r.ioStreams.Out, "inventoryID: %s is used for inventory object\n", r.InventoryID)
	return nil
}

// defaultInventoryID returns a UUID string as a default unique
// identifier for a inventory object label.
func (r *Config) defaultInventoryID() (string, error) {
	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// validateInventoryID returns true of the passed group name is a
// valid label value; false otherwise. The valid label values
// are [a-z0-9A-Z] "-", "_", and "." The inventoryID must not
// be empty, but it can not be more than 63 characters.
func validateInventoryID(inventoryID string) bool {
	if len(inventoryID) == 0 || len(inventoryID) > 63 {
		return false
	}
	re := regexp.MustCompile(inventoryIDRegexp)
	return re.MatchString(inventoryID)
}

func (r *Config) Run(ctx context.Context) error {
	log := log.FromContext(ctx)
	invPath := InventoryPath(r.Dir)
	if fsys.FileExists(invPath) {
		return fmt.Errorf("inventory object template file already exists: %s", invPath)
	}
	fsys.EnsureDir(ctx, filepath.Dir(invPath))
	log.Debug("creating inventory manifest", "filename", invPath)
	f, err := os.Create(invPath)
	if err != nil {
		return fmt.Errorf("unable to create inventory object template file: %s", err)
	}
	defer f.Close()
	_, err = f.WriteString(r.fillInValues(ctx))
	if err != nil {
		return fmt.Errorf("unable to write inventory object template file: %s", invPath)
	}
	fmt.Fprintf(r.ioStreams.Out, "Initialized: %s\n", invPath)
	return nil
}

// fillInValues returns a string of the inventory object template
// ConfigMap with values filled in (eg. namespace, inventoryID).
// TODO(seans3): Look into text/template package.
func (r *Config) fillInValues(_ context.Context) string {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05 MST")
	randomSuffix := common.RandomStr()
	manifestStr := r.Template
	manifestStr = strings.ReplaceAll(manifestStr, "<DATETIME>", nowStr)
	manifestStr = strings.ReplaceAll(manifestStr, "<NAMESPACE>", r.Namespace)
	manifestStr = strings.ReplaceAll(manifestStr, "<RANDOMSUFFIX>", randomSuffix)
	manifestStr = strings.ReplaceAll(manifestStr, "<INVENTORYID>", r.InventoryID)
	manifestStr = strings.ReplaceAll(manifestStr, "<INVENTORYKEY>", invv1alpha1.InventoryLabelKey)
	return manifestStr
}

func InventoryPath(path string) string {
	return filepath.Join(path, kformInventoryManifestFileSubDir, kformInventoryManifestFilename)
}

func GetInventoryInfo(path string) (*unstructured.Unstructured, error) {
	var u *unstructured.Unstructured
	invPath := InventoryPath(path)
	if !fsys.FileExists(invPath) {
		return u, fmt.Errorf("run kform init, inventory path not exist: %s", invPath)
	}
	b, err := os.ReadFile(invPath)
	if err != nil {
		return u, fmt.Errorf("run kform init, cannot read inv file: %s, err: %s", invPath, err.Error())
	}
	if err := yaml.Unmarshal(b, &u); err != nil {
		return u, err
	}
	return u, nil
}
