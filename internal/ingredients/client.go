// Package ingredients provides clients for Swedish food data sources:
// Livsmedelsverket (national food database) and Dabas (product database).
// These feed nutrition metadata and product-level detail into the
// planning and recommendation pipeline.
package ingredients

// Package-level import guard — ensures httpclient is available.
import _ "github.com/androidand/spisordning/internal/httpclient"
