package dependencies

import "strings"

// detectLicenseType returns license type from name or content
func detectLicenseType(nameOrContent string) string {
	// Normalize the license name/content
	normalized := strings.ToUpper(strings.TrimSpace(nameOrContent))

	// Common SPDX identifiers - check content patterns
	// Order matters: check more specific patterns first
	switch {
	case strings.Contains(normalized, "BSD 3-CLAUSE") || strings.Contains(normalized, "BSD-3-CLAUSE"):
		return "BSD-3-Clause"
	case strings.Contains(normalized, "REDISTRIBUTION AND USE") && strings.Contains(normalized, "THREE "):
		return "BSD-3-Clause"
	case strings.Contains(normalized, "BSD 2-CLAUSE") || strings.Contains(normalized, "BSD-2-CLAUSE"):
		return "BSD-2-Clause"
	case strings.Contains(normalized, "APACHE LICENSE") || strings.Contains(normalized, "APACHE 2.0"):
		return "Apache-2.0"
	case strings.Contains(normalized, "APACHE-2.0"):
		return "Apache-2.0"
	case strings.Contains(normalized, "GNU GENERAL PUBLIC LICENSE") && strings.Contains(normalized, "VERSION 3"):
		return "GPL-3.0"
	case strings.Contains(normalized, "GPL-3.0"):
		return "GPL-3.0"
	case strings.Contains(normalized, "GPL-2.0"):
		return "GPL-2.0"
	case strings.Contains(normalized, "GNU LESSER GENERAL PUBLIC LICENSE"):
		return "LGPL-3.0"
	case strings.Contains(normalized, "LGPL"):
		return "LGPL-3.0"
	case strings.Contains(normalized, "MOZILLA PUBLIC LICENSE"):
		return "MPL-2.0"
	case strings.Contains(normalized, "ISC LICENSE") || strings.Contains(normalized, "ISC"):
		return "ISC"
	case strings.Contains(normalized, "UNLICENSE"):
		return "Unlicense"
	case strings.Contains(normalized, "MIT LICENSE") || strings.Contains(normalized, "MIT"):
		return "MIT"
	default:
		return "Unknown"
	}
}

// getLicenseURL returns standard URL for license type
func getLicenseURL(licenseType string) string {
	switch licenseType {
	case "MIT":
		return "https://opensource.org/licenses/MIT"
	case "Apache-2.0":
		return "https://www.apache.org/licenses/LICENSE-2.0"
	case "BSD-3-Clause":
		return "https://opensource.org/licenses/BSD-3-Clause"
	case "BSD-2-Clause":
		return "https://opensource.org/licenses/BSD-2-Clause"
	case "GPL-3.0":
		return "https://www.gnu.org/licenses/gpl-3.0.html"
	case "GPL-2.0":
		return "https://www.gnu.org/licenses/gpl-2.0.html"
	case "LGPL-3.0":
		return "https://www.gnu.org/licenses/lgpl-3.0.html"
	case "ISC":
		return "https://opensource.org/licenses/ISC"
	case "MPL-2.0":
		return "https://www.mozilla.org/en-US/MPL/2.0/"
	case "Unlicense":
		return "http://unlicense.org/"
	default:
		return ""
	}
}
