terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">4.0"
    }
  }
}

provider "google" {
  project     = "fluid-house-332216"
  region      = "us-central1"
  zone    = "us-central1-c"
}