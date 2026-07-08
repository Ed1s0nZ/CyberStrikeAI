const DEFAULT_INSTRUCTION = CSAI_DEFAULT_INSTRUCTION;

function defaultInstruction() {
  return DEFAULT_INSTRUCTION;
}

function toPrompt(entry, instruction) {
  if (entry && entry.isPageContext) {
    const prefix = (instruction && instruction.trim()) ? instruction.trim() : DEFAULT_INSTRUCTION;
    return (
      prefix +
      '\n\n[Target]\n' +
      'PAGE ' +
      (entry.url || '') +
      '\n\n[Page]\n' +
      'Title: ' +
      (entry.pageTitle || '') +
      '\nURL: ' +
      (entry.url || '')
    );
  }
  const prefix = (instruction && instruction.trim()) ? instruction.trim() : DEFAULT_INSTRUCTION;
  const method = entry.method || 'GET';
  const url = entry.url || '(unknown)';
  const reqHeaders = normalizeRequestBlock(entry);
  const reqBody = entry.requestBody || '';
  let respSnippet = '';
  if (entry.responseHeaders || entry.responseBody) {
    respSnippet =
      '\n\n[Optional: Response (truncated)]\n' +
      normalizeResponseBlock(entry) +
      '\n\n' +
      (entry.responseBody || '');
  }
  return (
    prefix +
    '\n\n[Target]\n' +
    method +
    ' ' +
    url +
    '\n\n[Request]\n' +
    reqHeaders +
    '\n\n' +
    reqBody +
    respSnippet
  );
}
