package resourcemanager_test

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-google/google/acctest"
	"github.com/hashicorp/terraform-provider-google/google/envvar"
	"github.com/hashicorp/terraform-provider-google/google/services/resourcemanager"
	transport_tpg "github.com/hashicorp/terraform-provider-google/google/transport"
)

var (
	TestPrefix = "tf-test"
)

// Test that a Project resource can be created without an organization
func TestAccProject_createWithoutOrg(t *testing.T) {
	t.Parallel()

	creds := transport_tpg.MultiEnvSearch(envvar.CredsEnvVars)
	if strings.Contains(creds, "iam.gserviceaccount.com") {
		t.Skip("Service accounts cannot create projects without a parent. Requires user credentials.")
	}

	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			// This step creates a new project
			{
				Config: testAccProject_createWithoutOrg(pid),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid),
				),
			},
		},
	})
}

// Test that a Project resource can be created and an IAM policy
// associated
func TestAccProject_create(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			// This step creates a new project
			{
				Config: testAccProject(pid, org),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid),
				),
			},
		},
	})
}

// Test that a Project resource can be created with an associated
// billing account
func TestAccProject_billing(t *testing.T) {
	t.Parallel()
	org := envvar.GetTestOrgFromEnv(t)
	// This is a second billing account that can be charged, which is used only in this test to
	// verify that a project can update its billing account.
	envvar.SkipIfEnvNotSet(t, "GOOGLE_BILLING_ACCOUNT_2")
	billingId2 := os.Getenv("GOOGLE_BILLING_ACCOUNT_2")
	billingId := envvar.GetTestBillingAccountFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			// This step creates a new project with a billing account
			{
				Config: testAccProject_createBilling(pid, org, billingId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectHasBillingAccount(t, "google_project.acceptance", pid, billingId),
				),
			},
			// Make sure import supports billing account
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
			// Update to a different  billing account
			{
				Config: testAccProject_createBilling(pid, org, billingId2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectHasBillingAccount(t, "google_project.acceptance", pid, billingId2),
				),
			},
			// Unlink the billing account
			{
				Config: testAccProject(pid, org),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectHasBillingAccount(t, "google_project.acceptance", pid, ""),
				),
			},
		},
	})
}

// Test that a Project resource can be created with labels
func TestAccProject_labels(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_labels(pid, org, "test", "that"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectHasLabels(t, "google_project.acceptance", pid, map[string]string{"test": "that"}),
				),
			},
			// Make sure import supports labels
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy", "labels", "terraform_labels"},
			},
			// update project with labels
			{
				Config: testAccProject_labels(pid, org, "label", "label-value"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid),
					testAccCheckGoogleProjectHasLabels(t, "google_project.acceptance", pid, map[string]string{"label": "label-value"}),
				),
			},
			// update project delete labels
			{
				Config: testAccProject(pid, org),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid),
					testAccCheckGoogleProjectHasNoLabels(t, "google_project.acceptance", pid),
				),
			},
			{
				Config: testAccProject_labels(pid, org, "label", "label-value"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectHasLabels(t, "google_project.acceptance", pid, map[string]string{"label": "label-value"}),
				),
			},
		},
	})
}

func TestAccProject_deleteDefaultNetwork(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	billingId := envvar.GetTestBillingAccountFromEnv(t)
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_deleteDefaultNetwork(pid, org, billingId),
			},
		},
	})
}

func TestAccProject_parentFolder(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	folderDisplayName := TestPrefix + acctest.RandString(t, 10)
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_parentFolder(pid, folderDisplayName, org),
			},
		},
	})
}

func TestAccProject_migrateParent(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	folderDisplayName := TestPrefix + acctest.RandString(t, 10)
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_migrateParentFolder(pid, folderDisplayName, org),
			},
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
			{
				Config: testAccProject_migrateParentOrg(pid, folderDisplayName, org),
			},
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
			{
				Config: testAccProject_migrateParentFolder(pid, folderDisplayName, org),
			},
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
		},
	})
}

// Test that a Project resource can be created with tags
func TestAccProject_tags(t *testing.T) {
	t.Parallel()

	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	tagKey := acctest.BootstrapSharedTestOrganizationTagKey(t, "crm-projects-tagkey", nil)
	context := map[string]interface{}{
		"pid":           pid,
		"org":           envvar.GetTestOrgFromEnv(t),
		"tagKey":        tagKey,
		"tagValue":      acctest.BootstrapSharedTestOrganizationTagValue(t, "crm-projects-tagvalue", tagKey),
		"random_suffix": acctest.RandString(t, 10),
	}
	acctest.VcrTest(t, resource.TestCase{
		PreCheck: func() { acctest.AccTestPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {},
		},
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_tags(context),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid),
				),
			},
			// Make sure import supports tags
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tags", "deletion_policy"}, // we don't read tags back
			},
			// Update tags tries to replace project but fails due to deletion policy
			{
				Config:      testAccProject_withoutTags(context),
				ExpectError: regexp.MustCompile("deletion_policy"),
			},
			{
				Config: testAccProject_tagsAllowDestroy(context),
			},
		},
	})
}

func testAccCheckGoogleProjectExists(r, pid string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return fmt.Errorf("Not found: %s", r)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set")
		}

		projectId := fmt.Sprintf("projects/%s", pid)
		if rs.Primary.ID != projectId {
			return fmt.Errorf("Expected project %q to match ID %q in state", projectId, rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckGoogleProjectHasBillingAccount(t *testing.T, r, pid, billingId string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return fmt.Errorf("Not found: %s", r)
		}

		// State should match expected
		if rs.Primary.Attributes["billing_account"] != billingId {
			return fmt.Errorf("Billing ID in state (%s) does not match expected value (%s)", rs.Primary.Attributes["billing_account"], billingId)
		}

		// Actual value in API should match state and expected
		// Read the billing account
		config := acctest.GoogleProviderConfig(t)
		ba, err := config.NewBillingClient(config.UserAgent).Projects.GetBillingInfo(resourcemanager.PrefixedProject(pid)).Do()
		if err != nil {
			return fmt.Errorf("Error reading billing account for project %q: %v", resourcemanager.PrefixedProject(pid), err)
		}
		if billingId != strings.TrimPrefix(ba.BillingAccountName, "billingAccounts/") {
			return fmt.Errorf("Billing ID returned by API (%s) did not match expected value (%s)", ba.BillingAccountName, billingId)
		}
		return nil
	}
}

func testAccCheckGoogleProjectHasLabels(t *testing.T, r, pid string, expected map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return fmt.Errorf("Not found: %s", r)
		}

		// State should have the same number of labels
		if rs.Primary.Attributes["labels.%"] != strconv.Itoa(len(expected)) {
			return fmt.Errorf("Expected %d labels, got %s", len(expected), rs.Primary.Attributes["labels.%"])
		}

		// Actual value in API should match state and expected
		config := acctest.GoogleProviderConfig(t)

		found, err := config.NewResourceManagerClient(config.UserAgent).Projects.Get(pid).Do()
		if err != nil {
			return err
		}

		actual := found.Labels
		if !reflect.DeepEqual(actual, expected) {
			// Determine only the different attributes
			for k, v := range expected {
				if av, ok := actual[k]; ok && v == av {
					delete(expected, k)
					delete(actual, k)
				}
			}

			spewConf := spew.NewDefaultConfig()
			spewConf.SortKeys = true
			return fmt.Errorf(
				"Labels not equivalent. Difference is shown below. Top is actual, bottom is expected."+
					"\n\n%s\n\n%s",
				spewConf.Sdump(actual), spewConf.Sdump(expected),
			)
		}
		return nil
	}
}

func testAccCheckGoogleProjectHasNoLabels(t *testing.T, r, pid string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return fmt.Errorf("Not found: %s", r)
		}

		// State should have zero labels
		if v, ok := rs.Primary.Attributes["labels.%"]; ok && v != "0" {
			return fmt.Errorf("Expected 0 labels, got %s", rs.Primary.Attributes["labels.%"])
		}

		// Actual value in API should match state and expected
		config := acctest.GoogleProviderConfig(t)

		found, err := config.NewResourceManagerClient(config.UserAgent).Projects.Get(pid).Do()
		if err != nil {
			return err
		}

		spewConf := spew.NewDefaultConfig()
		spewConf.SortKeys = true
		if found.Labels != nil {
			return fmt.Errorf("Labels should be empty. Actual \n%s", spewConf.Sdump(found.Labels))
		}
		return nil
	}
}

func TestAccProject_noAllowDestroy(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_noAllowDestroy(pid, org),
			},
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
			{
				Config:      testAccProject_noAllowDestroy(pid, org),
				Destroy:     true,
				ExpectError: regexp.MustCompile("deletion_policy"),
			},
			{
				Config: testAccProject(pid, org),
			},
		},
	})
}

func TestAccProject_abandon(t *testing.T) {
	t.Parallel()

	org := envvar.GetTestOrgFromEnv(t)
	pid := fmt.Sprintf("%s-%d", TestPrefix, acctest.RandInt(t))
	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProject_abandon(pid, org),
			},
			{
				ResourceName:            "google_project.acceptance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"deletion_policy"},
			},
			{
				Config:  testAccProject_abandon(pid, org),
				Destroy: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGoogleProjectExists("google_project.acceptance", pid)),
			},
		},
	})
}

func testAccProject_createWithoutOrg(pid string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  deletion_policy = "DELETE"
}
`, pid, pid)
}

func testAccProject(pid, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  org_id = "%s"
  deletion_policy = "DELETE"
}
`, pid, pid, org)
}

func testAccProject_noAllowDestroy(pid, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  org_id = "%s"
  deletion_policy = "PREVENT"
}
`, pid, pid, org)
}

func testAccProject_abandon(pid, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  org_id = "%s"
  deletion_policy = "ABANDON"
}
`, pid, pid, org)
}

func testAccProject_createBilling(pid, org, billing string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id      = "%s"
  name            = "%s"
  org_id          = "%s"
  billing_account = "%s"
  deletion_policy = "DELETE"
}
`, pid, pid, org, billing)
}

func testAccProject_labels(pid, org, key, value string) string {
	return fmt.Sprintf(`
provider "google" {
  add_terraform_attribution_label = false
}

resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  org_id     = "%s"
  deletion_policy = "DELETE"
  labels = {
    "%s" = "%s"
  }
}
`, pid, pid, org, key, value)
}

func testAccProject_deleteDefaultNetwork(pid, org, billing string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id          = "%s"
  name                = "%s"
  org_id              = "%s"
  billing_account     = "%s" # requires billing to enable compute API
  deletion_policy = "DELETE"
  auto_create_network = false
}
`, pid, pid, org, billing)
}

func testAccProject_parentFolder(pid, folderName, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  deletion_policy = "DELETE"

  folder_id = google_folder.folder1.id
}

resource "google_folder" "folder1" {
  display_name = "%s"
  parent       = "organizations/%s"
  deletion_protection = false
}
`, pid, pid, folderName, org)
}

func testAccProject_migrateParentFolder(pid, folderName, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  deletion_policy = "DELETE"

  folder_id = google_folder.folder1.id
}

resource "google_folder" "folder1" {
  display_name = "%s"
  parent       = "organizations/%s"
  deletion_protection = false
}
`, pid, pid, folderName, org)
}

func testAccProject_migrateParentOrg(pid, folderName, org string) string {
	return fmt.Sprintf(`
resource "google_project" "acceptance" {
  project_id = "%s"
  name       = "%s"
  deletion_policy = "DELETE"

  org_id = "%s"
}

resource "google_folder" "folder1" {
  display_name = "%s"
  parent       = "organizations/%s"
  deletion_protection = false
}
`, pid, pid, org, folderName, org)
}

func testAccProject_tags(context map[string]interface{}) string {
	return acctest.Nprintf(`
resource "google_project" "acceptance" {
  project_id = "%{pid}"
  name       = "%{pid}"
  org_id     = "%{org}"
  tags = {
	"%{org}/%{tagKey}" = "%{tagValue}"
  }
}
`, context)
}

func testAccProject_withoutTags(context map[string]interface{}) string {
	return acctest.Nprintf(`
resource "google_project" "acceptance" {
  project_id = "%{pid}"
  name       = "%{pid}"
  org_id     = "%{org}"
}
`, context)
}

func testAccProject_tagsAllowDestroy(context map[string]interface{}) string {
	return acctest.Nprintf(`
resource "google_project" "acceptance" {
  project_id      = "%{pid}"
  name            = "%{pid}"
  org_id          = "%{org}"
  deletion_policy = "DELETE"
  tags            = {
	"%{org}/%{tagKey}" = "%{tagValue}"
  }
}
`, context)
}
