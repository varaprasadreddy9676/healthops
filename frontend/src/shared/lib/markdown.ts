const HORIZONTAL_RULE_RE = /^\s*([-_*])(?:\s*\1){2,}\s*$/

export function isMarkdownHorizontalRule(line: string): boolean {
  return HORIZONTAL_RULE_RE.test(line)
}

export function markdownToPlainText(text: string): string {
  return text
    .split('\n')
    .filter((line) => !isMarkdownHorizontalRule(line))
    .filter((line) => !line.trim().startsWith('```'))
    .join(' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/\s+/g, ' ')
    .trim()
}
