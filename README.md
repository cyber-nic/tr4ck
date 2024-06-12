# Tr4ck

Tr4ck is designed to help software development teams identify, track, and manage technical debt within their codebases. Tr4ck provides automated scanning, real-time insights, and actionable reports to ensure that technical debt is managed effectively, improving overall code quality and maintainability.

# Overview

Tr4ck is built to seamlessly integrate into your development workflow, providing continuous monitoring and actionable insights into your codebase's health. It supports multiple programming languages and provides customizable rule sets to adapt to your project's specific needs.

# Features

## Automated Scanning

Tr4ck integrates with your VCS to automatically scan for technical debt issues upon code commits, pull requests, and merges. It utilizes static code analysis tools to identify potential problems and categorizes them based on severity and impact.

## Keyword Detection

Our application can scan for specific keywords within code comments and documentation, such as "technical debt" or "Tr4ck". This helps in identifying and tracking known issues that have been documented by developers.

## Reporting and Dashboards

Detailed reports are generated for every scan, providing insights into the types and locations of technical debt. The dashboard offers a high-level overview and allows for drilling down into specific issues.

# CI/CD Integration

Tr4ck integrates with popular CI/CD tools to ensure continuous monitoring. It can enforce quality gates to prevent the accumulation of technical debt by failing builds that exceed predefined thresholds.

# Notification System

Stay informed with notifications sent via email, Slack, or integrated project management tools. Notifications can be configured based on severity levels and types of issues detected.

# License

Information about the project's licensing.

# Contact

How to get in touch with the Tr4ck support team or community.

# Basics

```
# list contents of the local registry
make run ARGS="reg ls"

# init the registry
make run ARGS="init"

# add a URL to the registry
make run ARGS="reg add https://github.com/cyber-nic/tr4ck"

# add a local path to the registry
make run ARGS="reg add /path/to/cyber-nic/tr4ck"

# scan a repo as-is
make run ARGS="scan https://github.com/cyber-nic/tr4ck"

# sync registered repos and scan since latest commit for tr4cks
make run ARGS=""

# provide custom config file
make run ARGS="--config=~/.tr4ck.conf reg ls"
```

# Configuration
Tr@ck has a built-in set of Markers, IgnoreDirs, and IgnoreExts. See below for details.

By default, Tr@ck will look for a custom yaml configuration file located here: `~/.track.conf`. If this file exist its content will override app defaults.

It is also possible to provide the location of a custom yaml configuration file using the parameter `--config=/path/to/file`. If this parameter is provided then default home directory location is ignored.

## Registry File Path
Tr@ck keeps things simple by storing state in a single file. This configuration can be overriden using the `registry_file_path` key.
Default: ~/.tr4ck.registry

## Markers
Terms to search for when identifying techincal debt. This configuration can be overriden using the `markers` key. 

Default:
  - tr@ck
  - todo
  - fixme

# IgnoreDirs
Directories to ignore. This configuration can be overriden using the `ignore_dirs` key.

Default:
  - .git
  - node_modules
  - .idea
  - .vscode
  - vendor
  - build
  - dist
  - .cache
  - target
  - .DS_Store
  - .svn
  - .hg
  - .tox
  - __pycache__
  - .mypy_cache
  - .pytest_cache

# IgnoreExts
File extensions to ignore. This configuration can be overriden using the `ignore_exts` key.

Default:
  - .json
  - .yaml
  - .yml
  - .sum
  - .mod
  - .html