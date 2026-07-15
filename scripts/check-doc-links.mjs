import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

function gitFiles(...args) {
  const output = execFileSync('git', ['-C', root, 'ls-files', ...args], { encoding: 'utf8' })
  return output.split('\n').map((line) => line.trim()).filter(Boolean)
}

const markdownFiles = [...new Set([
  ...gitFiles('--cached', '--', '*.md'),
  ...gitFiles('--others', '--exclude-standard', '--', '*.md'),
])].filter((file) => (
  file === 'DEV_GUIDE.md'
  || /^README(?:_[A-Z]+)?\.md$/.test(file)
  || file.startsWith('docs/')
  || (file.startsWith('deploy/') && !file.slice('deploy/'.length).includes('/'))
))

const failures = []
const linkPattern = /!?\[[^\]]*\]\(([^)]+)\)/g

for (const file of markdownFiles) {
  const absoluteFile = resolve(root, file)
  const source = readFileSync(absoluteFile, 'utf8').replace(/```[\s\S]*?```/g, '')
  for (const match of source.matchAll(linkPattern)) {
    let target = match[1].trim()
    if (target.startsWith('<') && target.endsWith('>')) target = target.slice(1, -1)
    target = target.split(/\s+["']/)[0]
    if (!target || /^(?:https?:|mailto:|tel:|data:|#)/i.test(target)) continue

    const pathOnly = target.split('#', 1)[0].split('?', 1)[0]
    if (!pathOnly) continue

    let decoded
    try {
      decoded = decodeURIComponent(pathOnly)
    } catch {
      failures.push(`${file}: invalid URL encoding in ${target}`)
      continue
    }

    const resolvedTarget = decoded.startsWith('/')
      ? resolve(root, decoded.slice(1))
      : resolve(dirname(absoluteFile), decoded)
    if (!existsSync(resolvedTarget)) {
      failures.push(`${file}: missing local target ${target}`)
    }
  }
}

const indexPath = resolve(root, 'docs/README.md')
if (!existsSync(indexPath)) {
  failures.push('docs/README.md: document index is missing')
} else {
  const index = readFileSync(indexPath, 'utf8')
  const currentDocs = markdownFiles.filter((file) => (
    file !== 'docs/README.md'
    && /^docs\/(?:[^/]+|legal\/[^/]+)\.md$/.test(file)
  ))
  for (const file of currentDocs) {
    const indexTarget = `./${relative(resolve(root, 'docs'), resolve(root, file)).split(sep).join('/')}`
    if (!index.includes(`](${indexTarget})`)) {
      failures.push(`${file}: not registered in docs/README.md as ${indexTarget}`)
    }
  }
}

if (failures.length > 0) {
  console.error('Documentation integrity check failed:')
  for (const failure of failures) console.error(`  - ${failure}`)
  process.exit(1)
}

console.log(`Documentation integrity check passed (${markdownFiles.length} files).`)
