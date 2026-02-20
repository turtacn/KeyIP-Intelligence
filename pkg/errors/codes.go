// Package errors provides centralized error code definitions for the KeyIP-Intelligence platform.
// All error codes are grouped by business domain and mapped to HTTP status codes.
package errors

import "net/http"

// ErrorCode represents a typed error code used throughout the KeyIP-Intelligence platform.
// Codes are partitioned by domain to avoid conflicts and simplify maintenance.
type ErrorCode int

// ─────────────────────────────────────────────────────────────────────────────
// General / cross-cutting error codes  (1xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodeOK indicates no error.
	CodeOK ErrorCode = 0

	// CodeUnknown is a catch-all for errors that have not been categorised.
	CodeUnknown ErrorCode = 10000

	// CodeInvalidParam is returned when one or more request parameters fail
	// validation (missing required fields, type mismatch, out-of-range values, etc.).
	CodeInvalidParam ErrorCode = 10001

	// CodeUnauthorized is returned when a request lacks valid authentication credentials.
	CodeUnauthorized ErrorCode = 10002

	// CodeForbidden is returned when authenticated credentials do not grant access
	// to the requested resource or action.
	CodeForbidden ErrorCode = 10003

	// CodeNotFound is returned when the requested resource does not exist.
	CodeNotFound ErrorCode = 10004

	// CodeConflict is returned when a create/update operation violates a uniqueness
	// or state constraint (e.g., duplicate resource, optimistic lock failure).
	CodeConflict ErrorCode = 10005

	// CodeRateLimit is returned when the caller has exceeded the allowed request rate.
	CodeRateLimit ErrorCode = 10006

	// CodeInternal is returned for unexpected server-side errors that are not
	// attributable to the caller.
	CodeInternal ErrorCode = 10007

	// CodeNotImplemented is returned when a requested feature or endpoint is
	// not yet implemented.
	CodeNotImplemented ErrorCode = 10008
)

// ─────────────────────────────────────────────────────────────────────────────
// Patent domain error codes  (2xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodePatentNotFound is returned when a patent with the requested identifier
	// cannot be located in any backing store (PostgreSQL, OpenSearch, etc.).
	CodePatentNotFound ErrorCode = 20001

	// CodePatentDuplicate is returned when an attempt is made to ingest or register
	// a patent that already exists (matched by publication number or canonical hash).
	CodePatentDuplicate ErrorCode = 20002

	// CodeClaimParseError is returned when the ClaimBERT model or claim parser
	// fails to parse or tokenise a patent claim text.
	CodeClaimParseError ErrorCode = 20003

	// CodeMarkushInvalid is returned when a Markush structure definition is
	// structurally invalid, incomplete, or cannot be expanded by the chemistry engine.
	CodeMarkushInvalid ErrorCode = 20004
)

// ─────────────────────────────────────────────────────────────────────────────
// Molecule domain error codes  (3xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodeMoleculeInvalidSMILES is returned when a provided SMILES string cannot
	// be parsed into a valid molecular graph.
	CodeMoleculeInvalidSMILES ErrorCode = 30001

	// CodeMoleculeNotFound is returned when a molecule with the requested
	// InChIKey, canonical SMILES, or internal ID cannot be located.
	CodeMoleculeNotFound ErrorCode = 30002

	// CodeFingerprintError is returned when fingerprint generation (Morgan, ECFP,
	// topological, etc.) fails for a given molecule.
	CodeFingerprintError ErrorCode = 30003

	// CodeSimilarityCalcError is returned when a pairwise or batch similarity
	// computation fails due to invalid inputs or a downstream model error.
	CodeSimilarityCalcError ErrorCode = 30004
)

// ─────────────────────────────────────────────────────────────────────────────
// Portfolio domain error codes  (4xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodePortfolioNotFound is returned when a patent portfolio with the requested
	// ID does not exist or is inaccessible to the current tenant.
	CodePortfolioNotFound ErrorCode = 40001

	// CodeValuationError is returned when the portfolio valuation algorithm fails
	// to compute a result (insufficient data, model error, numeric overflow, etc.).
	CodeValuationError ErrorCode = 40002
)

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle domain error codes  (5xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodeDeadlineMissed is returned when a lifecycle action (renewal, response,
	// grant fee payment) cannot be recorded because the statutory deadline has passed.
	CodeDeadlineMissed ErrorCode = 50001

	// CodeAnnuityCalcError is returned when the annuity fee calculation engine
	// fails to produce a result for a given jurisdiction and patent age.
	CodeAnnuityCalcError ErrorCode = 50002

	// CodeJurisdictionUnknown is returned when an unsupported or unrecognised
	// jurisdiction code is supplied to the lifecycle or annuity subsystem.
	CodeJurisdictionUnknown ErrorCode = 50003
)

// ─────────────────────────────────────────────────────────────────────────────
// Intelligence layer error codes  (6xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodeModelLoadError is returned when an AI model (GNN, BERT, GPT, etc.)
	// cannot be loaded from the model registry or serving backend.
	CodeModelLoadError ErrorCode = 60001

	// CodeInferenceTimeout is returned when a model inference call exceeds the
	// configured deadline (default: 30 s for online; 5 min for batch).
	CodeInferenceTimeout ErrorCode = 60002

	// CodeModelNotReady is returned when a request is made against a model that
	// has not yet completed warm-up or is undergoing a rolling update.
	CodeModelNotReady ErrorCode = 60003
)

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure error codes  (7xxxx)
// ─────────────────────────────────────────────────────────────────────────────
const (
	// CodeDBConnectionError is returned when the application cannot establish or
	// re-use a connection to PostgreSQL or Neo4j.
	CodeDBConnectionError ErrorCode = 70001

	// CodeDBQueryError is returned when a database query fails due to syntax
	// errors, constraint violations (not covered by CodeConflict), or other
	// execution-time failures.
	CodeDBQueryError ErrorCode = 70007

	// CodeDatabaseError is a general error for database-related failures that
	// are not specifically connection issues.
	CodeDatabaseError ErrorCode = 70006

	// CodeCacheError is returned when a Redis operation (GET, SET, DEL, EVAL, etc.)
	// fails due to connection loss, timeout, or an unexpected response.
	CodeCacheError ErrorCode = 70002

	// CodeSearchError is returned when an OpenSearch or Milvus query or indexing
	// operation fails.
	CodeSearchError ErrorCode = 70003

	// CodeMessageQueueError is returned when producing to or consuming from a
	// Kafka topic fails (broker unavailable, serialisation error, offset commit, etc.).
	CodeMessageQueueError ErrorCode = 70004

	// CodeStorageError is returned when a MinIO object storage operation (upload,
	// download, stat, delete) fails.
	CodeStorageError ErrorCode = 70005
)

// ─────────────────────────────────────────────────────────────────────────────
// String — human-readable name of the error code
// ─────────────────────────────────────────────────────────────────────────────

// String returns the human-readable name associated with an ErrorCode.
// It is safe to call on any value, including unknown codes.
func (c ErrorCode) String() string {
	switch c {
	// General
	case CodeOK:
		return "OK"
	case CodeUnknown:
		return "UNKNOWN"
	case CodeInvalidParam:
		return "INVALID_PARAM"
	case CodeUnauthorized:
		return "UNAUTHORIZED"
	case CodeForbidden:
		return "FORBIDDEN"
	case CodeNotFound:
		return "NOT_FOUND"
	case CodeConflict:
		return "CONFLICT"
	case CodeRateLimit:
		return "RATE_LIMIT"
	case CodeInternal:
		return "INTERNAL_ERROR"
	case CodeNotImplemented:
		return "NOT_IMPLEMENTED"

	// Patent
	case CodePatentNotFound:
		return "PATENT_NOT_FOUND"
	case CodePatentDuplicate:
		return "PATENT_DUPLICATE"
	case CodeClaimParseError:
		return "CLAIM_PARSE_ERROR"
	case CodeMarkushInvalid:
		return "MARKUSH_INVALID"

	// Molecule
	case CodeMoleculeInvalidSMILES:
		return "MOLECULE_INVALID_SMILES"
	case CodeMoleculeNotFound:
		return "MOLECULE_NOT_FOUND"
	case CodeFingerprintError:
		return "FINGERPRINT_ERROR"
	case CodeSimilarityCalcError:
		return "SIMILARITY_CALC_ERROR"

	// Portfolio
	case CodePortfolioNotFound:
		return "PORTFOLIO_NOT_FOUND"
	case CodeValuationError:
		return "VALUATION_ERROR"

	// Lifecycle
	case CodeDeadlineMissed:
		return "DEADLINE_MISSED"
	case CodeAnnuityCalcError:
		return "ANNUITY_CALC_ERROR"
	case CodeJurisdictionUnknown:
		return "JURISDICTION_UNKNOWN"

	// Intelligence
	case CodeModelLoadError:
		return "MODEL_LOAD_ERROR"
	case CodeInferenceTimeout:
		return "INFERENCE_TIMEOUT"
	case CodeModelNotReady:
		return "MODEL_NOT_READY"

	// Infrastructure
	case CodeDBConnectionError:
		return "DB_CONNECTION_ERROR"
	case CodeDBQueryError:
		return "DB_QUERY_ERROR"
	case CodeDatabaseError:
		return "DATABASE_ERROR"
	case CodeCacheError:
		return "CACHE_ERROR"
	case CodeSearchError:
		return "SEARCH_ERROR"
	case CodeMessageQueueError:
		return "MESSAGE_QUEUE_ERROR"
	case CodeStorageError:
		return "STORAGE_ERROR"

	default:
		return "UNKNOWN_CODE"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTPStatus — mapping from domain error codes to HTTP status codes
// ─────────────────────────────────────────────────────────────────────────────

// HTTPStatus returns the most appropriate HTTP status code for the given ErrorCode.
// The mapping follows RFC 9110 semantics and is used by HTTP handlers in
// internal/interfaces/http/handlers/ to translate domain errors into HTTP responses.
//
// Decision matrix:
//   - 200 OK              → CodeOK
//   - 400 Bad Request     → CodeInvalidParam, CodeMarkushInvalid, CodeMoleculeInvalidSMILES, CodeClaimParseError
//   - 401 Unauthorized    → CodeUnauthorized
//   - 403 Forbidden       → CodeForbidden
//   - 404 Not Found       → CodeNotFound, CodePatentNotFound, CodeMoleculeNotFound, CodePortfolioNotFound
//   - 409 Conflict        → CodeConflict, CodePatentDuplicate
//   - 429 Too Many Req.   → CodeRateLimit
//   - 503 Service Unavail → CodeModelNotReady, CodeDBConnectionError, CodeMessageQueueError
//   - 504 Gateway Timeout → CodeInferenceTimeout
//   - 500 Internal Server → everything else
func (c ErrorCode) HTTPStatus() int {
	switch c {
	case CodeOK:
		return http.StatusOK

	case CodeInvalidParam,
		CodeMarkushInvalid,
		CodeMoleculeInvalidSMILES,
		CodeClaimParseError,
		CodeJurisdictionUnknown:
		return http.StatusBadRequest

	case CodeUnauthorized:
		return http.StatusUnauthorized

	case CodeForbidden:
		return http.StatusForbidden

	case CodeNotFound,
		CodePatentNotFound,
		CodeMoleculeNotFound,
		CodePortfolioNotFound:
		return http.StatusNotFound

	case CodeConflict,
		CodePatentDuplicate,
		CodeDeadlineMissed:
		return http.StatusConflict

	case CodeRateLimit:
		return http.StatusTooManyRequests

	case CodeModelNotReady,
		CodeDBConnectionError,
		CodeMessageQueueError,
		CodeStorageError:
		return http.StatusServiceUnavailable

	case CodeDBQueryError:
		return http.StatusInternalServerError

	case CodeNotImplemented:
		return http.StatusNotImplemented

	case CodeInferenceTimeout:
		return http.StatusGatewayTimeout

	default:
		// CodeUnknown, CodeInternal, CodeFingerprintError, CodeSimilarityCalcError,
		// CodeValuationError, CodeAnnuityCalcError, CodeModelLoadError,
		// CodeCacheError, CodeSearchError, and all unrecognised codes.
		return http.StatusInternalServerError
	}
}

