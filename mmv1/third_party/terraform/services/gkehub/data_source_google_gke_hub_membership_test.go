package gkehub_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-google/google/acctest"
	"github.com/hashicorp/terraform-provider-google/google/envvar"
	"github.com/hashicorp/terraform-provider-google/google/tpgresource"
	transport_tpg "github.com/hashicorp/terraform-provider-google/google/transport"
)

func TestAccDataSourceGoogleGkeHubMembership_basic(t *testing.T) {
	t.Parallel()

	project := envvar.GetTestProjectFromEnv()
	gkeClusterRegion := "us-central1"
	gkeClusterZone := "us-central1-a"
	membershipLocation := "global"
	randomSuffix := acctest.RandString(t, 10)

	// Define unique names for network and subnetwork for this test run
	networkName := fmt.Sprintf("tf-test-mem-ds-net-%s", randomSuffix)
	subnetworkName := fmt.Sprintf("tf-test-mem-ds-sub-%s", randomSuffix)

	context := map[string]interface{}{
		"project":             project,
		"gke_cluster_region":  gkeClusterRegion,
		"gke_cluster_zone":    gkeClusterZone,
		"membership_location": membershipLocation,
		"random_suffix":       randomSuffix,
		"network_name":        networkName,
		"subnetwork_name":     subnetworkName,
	}

	acctest.VcrTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccTestPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories(t),
		CheckDestroy:             testAccCheckGoogleGkeHubMembershipDestroyProducer(t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceGoogleGkeHubMembership_basic_config(context),
				Check: resource.ComposeTestCheckFunc(
					acctest.CheckDataSourceStateMatchesResourceState("data.google_gke_hub_membership.example", "google_gke_hub_membership.example"),
				),
			},
		},
	})
}

func testAccDataSourceGoogleGkeHubMembership_basic_config(context map[string]interface{}) string {
	return acctest.Nprintf(`
resource "google_compute_network" "default" {
  project                 = "%{project}"
  name                    = "%{network_name}"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "default" {
  project       = "%{project}"
  name          = "%{subnetwork_name}"
  ip_cidr_range = "10.2.0.0/16" // Example CIDR
  region        = "%{gke_cluster_region}"
  network       = google_compute_network.default.id
}

resource "google_container_cluster" "primary" {
  project               = "%{project}"
  name                  = "tf-test-mem-ds-cl-%{random_suffix}"
  location              = "%{gke_cluster_zone}"
  initial_node_count    = 1
  deletion_protection   = false
  network               = google_compute_network.default.id
  subnetwork            = google_compute_subnetwork.default.id

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }
}

resource "google_gke_hub_membership" "example" {
  project       = "%{project}"
  membership_id = "tf-test-mem-%{random_suffix}"
  location      = "%{membership_location}"

  endpoint {
    gke_cluster {
      resource_link = "//container.googleapis.com/${google_container_cluster.primary.id}"
    }
  }

  depends_on = [google_container_cluster.primary]
}

data "google_gke_hub_membership" "example" {
  project       = google_gke_hub_membership.example.project
  location      = google_gke_hub_membership.example.location
  membership_id = google_gke_hub_membership.example.membership_id
}
`, context)
}

func testAccCheckGoogleGkeHubMembershipDestroyProducer(t *testing.T) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "google_gke_hub_membership" {
				continue
			}
			if strings.HasPrefix(name, "data.") {
				continue
			}

			config := acctest.GoogleProviderConfig(t)
			url, err := tpgresource.ReplaceVarsForTest(config, rs, "{{GKEHub2BasePath}}projects/{{project}}/locations/{{location}}/memberships/{{membership_id}}")
			if err != nil {
				return fmt.Errorf("Error constructing URL for GKE Hub Membership: %s", err)
			}

			billingProject := ""

			if config.BillingProject != "" {
				billingProject = config.BillingProject
			}

			_, err = transport_tpg.SendRequest(transport_tpg.SendRequestOptions{
				Config:    config,
				Method:    "GET",
				RawURL:    url,
				UserAgent: config.UserAgent,
				Project:   billingProject,
			})

			if err == nil {
				return fmt.Errorf("GKEHubMembership still exists at %s", url)
			}
		}
		return nil
	}
}
