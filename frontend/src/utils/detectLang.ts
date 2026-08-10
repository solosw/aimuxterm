const langMap: Record<string, string> = {
  // JS/TS family
  ts: 'typescript', tsx: 'typescript', mts: 'typescript', cts: 'typescript',
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  // Web
  html: 'xml', htm: 'xml', vue: 'vue', svelte: 'xml',
  css: 'css', scss: 'scss', sass: 'scss', less: 'less',
  // Data
  json: 'json', jsonc: 'json', json5: 'json',
  xml: 'xml', svg: 'xml', yaml: 'yaml', yml: 'yaml', toml: 'ini',
  // Markup
  md: 'markdown', mdx: 'markdown', markdown: 'markdown',
  // C family
  c: 'c', h: 'c', cpp: 'cpp', cxx: 'cpp', cc: 'cpp', hpp: 'cpp', hxx: 'cpp', 'c++': 'cpp',
  // JVM
  java: 'java', kt: 'kotlin', kts: 'kotlin', scala: 'scala', groovy: 'groovy',
  // Go
  go: 'go',
  // Rust
  rs: 'rust',
  // Python
  py: 'python', pyw: 'python', ipynb: 'python',
  // Ruby
  rb: 'ruby', rake: 'ruby', gemspec: 'ruby',
  // Swift / ObjC
  swift: 'swift', m: 'objectivec', mm: 'objectivec',
  // PHP
  php: 'php', phtml: 'xml',
  // C#
  cs: 'csharp',
  // Lua
  lua: 'lua',
  // R
  r: 'r', rmd: 'r',
  // Dart / Flutter
  dart: 'dart',
  // Shell
  sh: 'bash', bash: 'bash', zsh: 'bash', fish: 'fish',
  bat: 'dos', cmd: 'dos', ps1: 'powershell', ps1m: 'powershell',
  // SQL
  sql: 'sql', pgsql: 'pgsql', mysql: 'sql', sqlite: 'sql',
  // Config
  ini: 'ini', cfg: 'ini', conf: 'ini', properties: 'ini', env: 'ini',
  dockerfile: 'dockerfile', dockerignore: 'dockerfile',
  gitignore: 'ini',
  makefile: 'makefile', cmake: 'cmake',
  // GraphQL
  gql: 'graphql', graphql: 'graphql',
  // Other
  elm: 'elm', erl: 'erlang', ex: 'elixir', exs: 'elixir',
  hs: 'haskell', lhs: 'haskell',
  clj: 'clojure', cljs: 'clojure', edn: 'clojure',
  f90: 'fortran', f95: 'fortran', f: 'fortran',
  pl: 'perl', pm: 'perl',
  scm: 'scheme', ss: 'scheme',
  proto: 'protobuf',
  tex: 'latex',
  vim: 'vim', vimrc: 'vim',
}

export function detectLang(filePath: string): string {
  const ext = (filePath.split('.').pop() || '').toLowerCase()
  // Special filenames
  const base = filePath.replace(/\\/g, '/').split('/').pop()?.toLowerCase() || ''
  if (base === 'dockerfile') return 'dockerfile'
  if (base === 'makefile') return 'makefile'
  if (base === '.gitignore') return 'ini'
  return langMap[ext] || 'plaintext'
}
