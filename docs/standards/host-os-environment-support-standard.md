# Host OS Environment Support Standard

**Standard**: Guidelines for cross-platform compatibility and OS-specific support tiers in goneat

## Overview

Goneat is designed as a cross-platform development tool that supports Windows, macOS, and Linux environments. This standard defines our operating system support tiers, platform-specific considerations, and compatibility guarantees. Our goal is to provide a consistent experience across all supported platforms while maintaining the highest possible functionality.

## Support Tiers

### Tier 1: Primary Support (Full Functionality)

**Platforms**: Windows 11 (x64), macOS (Intel/Apple Silicon), Ubuntu 22.04+ LTS, Debian 12+ LTS

#### Operating System Requirements

- **Windows 11 (x64)**: Windows 11 version 22H2 or later
- **macOS**: macOS 12.0 Monterey or later (last 3 major releases)
- **Ubuntu**: Ubuntu 22.04 LTS (Jammy Jellyfish) or later LTS releases
- **Debian**: Debian 12 (Bookworm) or later stable releases

#### Compatibility Guarantees

- **Full Feature Support**: All goneat features work as documented
- **Automated Testing**: All tests run successfully in CI/CD pipelines
- **Regular Validation**: Monthly verification of functionality
- **Security Updates**: Immediate patching for security issues
- **Bug Fixes**: High priority for reported issues

### Tier 2: Extended Support (Limited Functionality)

**Platforms**: Windows 10 (x64), macOS 11.x, Ubuntu 20.04 LTS, Debian 11

#### Compatibility Guarantees

- **Core Features**: Basic functionality works as documented
- **Manual Testing**: Quarterly validation of functionality
- **Bug Fixes**: Best-effort basis for critical issues
- **Security Updates**: Applied as feasible

### Tier 3: Experimental Support

**Platforms**: Windows 11 ARM64, Windows on ARM, Other Linux distributions

#### Compatibility Guarantees

- **Best Effort**: Functionality may work but is not guaranteed
- **Community Support**: Issues handled on best-effort basis
- **No CI/CD Testing**: Not included in automated test suites
- **Documentation**: Marked as experimental in user guides

## Platform-Specific Considerations

### Windows Support

#### Git Hooks

- **PowerShell**: Preferred on Windows 11 with PowerShell 7.0+
- **CMD**: Fallback for systems without PowerShell 7.0+
- **Git for Windows**: Required for Git operations
- **Path Handling**: Automatic conversion between Windows and Unix-style paths

#### Environment Variables

- **GONEAT_HOME**: Resolves to `%USERPROFILE%\.goneat`
- **Path Separators**: Automatic handling of `\` vs `/`
- **Case Sensitivity**: Case-insensitive file operations where appropriate

#### File System Operations

- **Permissions**: Windows ACL support for file access
- **Symbolic Links**: Support for Windows symbolic links and junctions
- **Long Paths**: Support for Windows long path names (>260 characters)
- **File Locking**: Appropriate locking mechanisms for concurrent access

### macOS Support

#### Git Hooks

- **bash/zsh**: Native shell support with automatic detection
- **Homebrew**: Optional but recommended for additional tools
- **Xcode Command Line Tools**: Required for some development features

#### Environment Variables

- **GONEAT_HOME**: Resolves to `$HOME/.goneat`
- **Path Handling**: Native Unix-style paths
- **Permissions**: Unix-style file permissions with extended attributes

#### File System Operations

- **APFS**: Full support for Apple's file system
- **Time Machine**: Proper exclusion of goneat directories
- **Spotlight**: Appropriate metadata handling
- **File Vault**: Compatibility with encrypted home directories

### Linux Support

#### Git Hooks

- **bash**: Primary shell support
- **Distribution Detection**: Automatic adaptation to different Linux distributions
- **Package Managers**: Support for apt, yum, dnf, pacman, etc.

#### Environment Variables

- **GONEAT_HOME**: Resolves to `$HOME/.goneat` or `$XDG_CONFIG_HOME/goneat` (if set)
- **Path Handling**: Native Unix-style paths
- **XDG Base Directory**: Support for XDG specification compliance

#### File System Operations

- **ext4/btrfs**: Full support for common Linux file systems
- **Permissions**: Unix-style file permissions with ACL support
- **SELinux/AppArmor**: Compatibility with security frameworks
- **File System Events**: Support for inotify-based file monitoring

## Cross-Platform Features

### Git Hook Templates

- **Automatic Detection**: OS detection at hook generation time
- **Template Selection**: Appropriate shell template based on detected OS
- **Fallback Support**: Graceful degradation when preferred shells unavailable
- **Path Resolution**: Cross-platform path handling in templates

### Configuration Files

- **YAML/JSON**: Universal format support across all platforms
- **Path Resolution**: Automatic path conversion and normalization
- **Encoding**: UTF-8 support with fallback to system encoding
- **Permissions**: Appropriate file permissions for each platform

### Command Execution

- **Shell Detection**: Automatic detection of available shells
- **Path Resolution**: Cross-platform executable path resolution
- **Environment Inheritance**: Proper environment variable handling
- **Signal Handling**: Platform-appropriate signal handling

### Network Operations

- **Proxy Support**: System proxy configuration detection
- **Certificate Stores**: Platform-specific certificate handling
- **DNS Resolution**: Platform-appropriate DNS configuration
- **Timeouts**: Appropriate timeout values for each platform

## Development Environment Support

### Integrated Development Environments

- **Visual Studio Code**: Full support with extensions
- **Visual Studio**: Windows-specific support
- **Xcode**: macOS-specific support
- **JetBrains IDEs**: Cross-platform support

### Container Environments

- **Docker Desktop**: Windows and macOS support
- **Podman**: Linux and macOS support
- **WSL2**: Windows Subsystem for Linux support
- **Dev Containers**: VS Code dev container support

### CI/CD Platforms

- **GitHub Actions**: Full support for all platforms
- **GitLab CI**: Full support for Linux, partial Windows/macOS
- **Azure DevOps**: Full Windows support, partial Linux/macOS
- **Jenkins**: Cross-platform support via agents

## Testing and Validation

### Automated Testing

- **Unit Tests**: Platform-agnostic test suites
- **Integration Tests**: Platform-specific test scenarios
- **End-to-End Tests**: Full workflow testing on each platform
- **Performance Tests**: Platform-specific performance validation

### Manual Validation

- **Release Testing**: Manual testing on all tier 1 platforms before release
- **Compatibility Testing**: Regular testing of supported software versions
- **User Environment Testing**: Testing in various user configurations

### Continuous Integration

- **Multi-Platform CI**: Tests run on all supported platforms
- **Matrix Testing**: Testing combinations of OS, architecture, and Go versions
- **Nightly Builds**: Automated builds for all supported platforms

## Migration and Compatibility

### Version Compatibility

- **Backward Compatibility**: Support for configuration files from previous versions
- **Migration Tools**: Automated migration scripts for configuration changes
- **Deprecation Warnings**: Clear warnings for deprecated features

### Feature Availability

- **Graceful Degradation**: Features work optimally on preferred platforms but degrade gracefully
- **Feature Detection**: Runtime detection of platform capabilities
- **Alternative Implementations**: Platform-specific implementations where needed

## Support and Maintenance

### Issue Classification

- **Platform-Specific**: Issues clearly tagged by affected platform
- **Priority Assignment**: Higher priority for tier 1 platform issues
- **Reproduction Requirements**: Clear reproduction steps for all platforms

### Documentation

- **Platform-Specific Guides**: Installation and usage guides for each platform
- **Troubleshooting**: Platform-specific troubleshooting guides
- **Known Limitations**: Clear documentation of platform limitations

### Community Support

- **Platform Communities**: Dedicated support channels for major platforms
- **User Contributions**: Encouragement of platform-specific contributions
- **Feedback Integration**: Regular collection of platform-specific feedback

## Future Considerations

### Platform Expansion

- **New OS Versions**: Support for new major OS releases within 3 months
- **New Platforms**: Evaluation framework for adding new platform support
- **Hardware Platforms**: Support for new CPU architectures (ARM64, RISC-V)

### Feature Evolution

- **Platform-Specific Features**: Features that leverage platform-specific capabilities
- **Cross-Platform Abstractions**: Higher-level abstractions for cross-platform development
- **Performance Optimizations**: Platform-specific performance improvements

---

**Last Updated**: September 10, 2025
**Supported Platforms**: Windows 11 (x64), macOS 12+, Ubuntu 22.04+, Debian 12+
**Experimental**: Windows 11 ARM64, Windows on ARM

**Co-Authored-By**: Forge Neat <noreply@3leaps.net>
**Generated by**: Forge Neat under supervision of @3leapsdave
