#!/usr/bin/env bash
# Prints the path of a Terraform CLI config that shares one provider plugin
# cache across every acceptance test in a run, creating both under the given
# build root. Each test runs in a fresh working directory with no lock file,
# and Terraform 1.4+ refuses a cached package without a lock entry unless the
# may-break flag is set, so a bare TF_PLUGIN_CACHE_DIR still downloads the
# released provider once per test. The config carries nothing that redirects
# provider installation, so the provider-upgrade entry's guard accepts it.
set -euo pipefail

root="${1:?build root}"
cache="$root/terraform-plugin-cache"
config="$root/terraform-plugin-cache.tfrc"
mkdir -p "$cache"
printf 'plugin_cache_dir = "%s"\nplugin_cache_may_break_dependency_lock_file = true\n' "$cache" > "$config"
printf '%s\n' "$config"
