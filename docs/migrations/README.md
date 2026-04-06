# Migrations

As the grafana-app-sdk is still both pre-1.0 and _experimental_, breaking changes may be introduced on minor version upgrades. 
This directory contains notes on the breaking changes (which are also typically in release notes on the first patch version 
of the minor version bump, for example, see [the v0.15.0 release notes](https://github.com/grafana/grafana-app-sdk/releases/tag/v0.15.0)), 
and instructions on what to do to migrate your project to the new version. Scripts may be included for more complex upgrades, if and when they occur. 

Minor version upgrades without a migration doc have no breaking changes or changes which are wholly handled by the `grafana-app-sdk generate` command.

## Automated migration tooling

For SDK version upgrades that involve breaking Go API changes, use the built-in migrate command to automate source code transforms (codemods):

```bash
# Preview changes without writing any files
grafana-app-sdk migrate --from v0.27.0 --to v0.29.0 --dry-run

# Apply codemods
grafana-app-sdk migrate --from v0.27.0 --to v0.29.0

# List all available codemods
grafana-app-sdk migrate --list-codemods
```

You can also scan for deprecated symbols before upgrading:

```bash
grafana-app-sdk lint --deprecations
```

And check SDK/Grafana version compatibility as a CI gate:

```bash
grafana-app-sdk compat --grafana-version 10.4
```

## Index

* [v0.14.x → v0.15.x](v0.15.md)
* [v0.25+ → v0.26.x (kind registry import)](v0.26.md)
* [v0.25+ → v0.27.x](v0.27.md)
* [v0.27.x → v0.28.x](v0.28.md)
* [v0.28+ → v0.30.x](v0.30.md)
* [v0.30+ → v0.32.x](v0.32.md)
* [v0.39+ → v0.40.x](v0.40.md)
* [v0.51+ → v0.52.x](v0.52.md)
