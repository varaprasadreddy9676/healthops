import type { ReactNode } from 'react'

import { isMarkdownHorizontalRule } from '@/shared/lib/markdown'
import { cn } from '@/shared/lib/utils'

interface MarkdownContentProps {
  text: string
  className?: string
}

const HEADING_RE = /^(#{1,6})\s+(.+)$/
const CODE_FENCE_RE = /^```/
const UNORDERED_LIST_RE = /^\s*[-*]\s+(.+)$/
const ORDERED_LIST_RE = /^\s*\d+\.\s+(.+)$/
const INLINE_TOKEN_RE = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*)/g

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = []
  let lastIndex = 0

  for (const match of text.matchAll(INLINE_TOKEN_RE)) {
    if (match.index == null) {
      continue
    }
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index))
    }

    const token = match[0]
    const key = `${keyPrefix}-${match.index}`
    if (token.startsWith('**')) {
      nodes.push(<strong key={key}>{token.slice(2, -2)}</strong>)
    } else if (token.startsWith('*')) {
      nodes.push(<em key={key}>{token.slice(1, -1)}</em>)
    } else if (token.startsWith('`')) {
      nodes.push(
        <code key={key} className="rounded bg-slate-100 px-1 py-0.5 text-[0.9em] dark:bg-slate-800">
          {token.slice(1, -1)}
        </code>,
      )
    }
    lastIndex = match.index + token.length
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex))
  }

  return nodes.length > 0 ? nodes : [text]
}

export function MarkdownContent({ text, className }: MarkdownContentProps) {
  const lines = text.split('\n')
  const blocks: ReactNode[] = []

  for (let i = 0; i < lines.length;) {
    const line = lines[i]
    const trimmed = line.trim()

    if (!trimmed) {
      i++
      continue
    }

    if (isMarkdownHorizontalRule(trimmed)) {
      blocks.push(<hr key={`hr-${i}`} className="border-slate-200 dark:border-slate-700" />)
      i++
      continue
    }

    if (CODE_FENCE_RE.test(trimmed)) {
      const start = i
      const codeLines: string[] = []
      i++
      for (; i < lines.length; i++) {
        if (CODE_FENCE_RE.test(lines[i].trim())) {
          i++
          break
        }
        codeLines.push(lines[i])
      }
      blocks.push(
        <pre key={`code-${start}`} className="overflow-x-auto rounded-lg bg-slate-900 px-3 py-2 text-xs leading-relaxed text-slate-100 dark:bg-slate-950">
          <code>{codeLines.join('\n')}</code>
        </pre>,
      )
      continue
    }

    const headingMatch = trimmed.match(HEADING_RE)
    if (headingMatch) {
      const level = headingMatch[1].length
      const content = renderInline(headingMatch[2], `h-${i}`)
      const baseClassName = 'font-semibold text-slate-900 dark:text-slate-100'
      if (level === 1) {
        blocks.push(<h1 key={`h-${i}`} className={cn('text-lg', baseClassName)}>{content}</h1>)
      } else if (level === 2) {
        blocks.push(<h2 key={`h-${i}`} className={cn('text-base', baseClassName)}>{content}</h2>)
      } else {
        blocks.push(<h3 key={`h-${i}`} className={cn('text-sm', baseClassName)}>{content}</h3>)
      }
      i++
      continue
    }

    const unorderedMatch = trimmed.match(UNORDERED_LIST_RE)
    if (unorderedMatch) {
      const start = i
      const items: ReactNode[] = []
      for (; i < lines.length; i++) {
        const listMatch = lines[i].trim().match(UNORDERED_LIST_RE)
        if (!listMatch) {
          break
        }
        items.push(<li key={`ul-${i}`}>{renderInline(listMatch[1], `ul-${i}`)}</li>)
      }
      blocks.push(<ul key={`ul-block-${start}`} className="list-disc space-y-1 pl-5">{items}</ul>)
      continue
    }

    const orderedMatch = trimmed.match(ORDERED_LIST_RE)
    if (orderedMatch) {
      const start = i
      const items: ReactNode[] = []
      for (; i < lines.length; i++) {
        const listMatch = lines[i].trim().match(ORDERED_LIST_RE)
        if (!listMatch) {
          break
        }
        items.push(<li key={`ol-${i}`}>{renderInline(listMatch[1], `ol-${i}`)}</li>)
      }
      blocks.push(<ol key={`ol-block-${start}`} className="list-decimal space-y-1 pl-5">{items}</ol>)
      continue
    }

    const start = i
    const paragraphLines: string[] = []
    for (; i < lines.length; i++) {
      const candidate = lines[i].trim()
      if (!candidate || isMarkdownHorizontalRule(candidate) || CODE_FENCE_RE.test(candidate) || HEADING_RE.test(candidate) || UNORDERED_LIST_RE.test(candidate) || ORDERED_LIST_RE.test(candidate)) {
        break
      }
      paragraphLines.push(candidate)
    }
    blocks.push(
      <p key={`p-${start}`} className="leading-relaxed">
        {renderInline(paragraphLines.join(' '), `p-${start}`)}
      </p>,
    )
  }

  return <div className={cn('space-y-3 text-sm text-slate-700 dark:text-slate-300', className)}>{blocks}</div>
}
