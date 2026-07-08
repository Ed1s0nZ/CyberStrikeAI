/** Shared limits and defaults for the browser extension. */
const CSAI_LIMITS = {
  MAX_CAPTURED: 200,
  MAX_RUNS: 50,
  MAX_REQUEST_BODY: 65536,
  MAX_RESPONSE_BODY: 4096,
  /** Progress log only; active Final Response is not truncated. */
  MAX_PROGRESS_CHARS: 524288,
  MAX_TAB_CAPTURES: 20,
  /** Markdown render skipped above this size (plain text only). */
  MAX_MARKDOWN_CHARS: 100000,
  /** Non-selected completed runs: soft-trim final to limit memory. */
  MAX_FINAL_ARCHIVE_CHARS: 100000,
};

const CSAI_DEFAULT_INSTRUCTION =
  'Perform web penetration testing on this traffic and output results. Test only this endpoint; do not expand to other APIs.';
