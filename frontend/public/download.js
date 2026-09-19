// Renders the public share folder. nginx serves /download/ with
// `autoindex_format json`, so the listing arrives as data and this page only
// has to present it. Kept in its own file rather than inline in download.html:
// the site's CSP is script-src 'self', which blocks inline scripts.

const out = document.getElementById('out')
const crumbs = document.getElementById('crumbs')

// Current folder, relative to the share root. Held in the URL hash so a
// subfolder can be linked and the back button works.
function currentPath() {
  const raw = decodeURIComponent(location.hash.replace(/^#/, ''))
  // Never let a hash walk out of the share root; nginx would refuse anyway,
  // but this keeps the links we build honest.
  return raw.split('/').filter((p) => p && p !== '.' && p !== '..').join('/')
}

function humanSize(bytes) {
  if (typeof bytes !== 'number') return ''
  const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`
}

function humanDate(mtime) {
  const d = new Date(mtime)
  return isNaN(d) ? '' : d.toLocaleString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
}

// Each path segment is encoded separately so slashes stay real separators.
function urlFor(path) {
  return '/download/' + path.split('/').map(encodeURIComponent).join('/')
}

const FOLDER_ICON =
  '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />'
const FILE_ICON =
  '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5" />'

function icon(shape) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
  svg.setAttribute('width', '18')
  svg.setAttribute('height', '18')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('fill', 'none')
  svg.setAttribute('stroke', 'currentColor')
  svg.setAttribute('stroke-width', '1.6')
  svg.setAttribute('stroke-linecap', 'round')
  svg.setAttribute('stroke-linejoin', 'round')
  svg.innerHTML = shape
  return svg
}

function renderCrumbs(path) {
  crumbs.textContent = ''
  const root = document.createElement('a')
  root.href = '#'
  root.textContent = 'Download'
  crumbs.append(root)
  let acc = ''
  for (const part of path ? path.split('/') : []) {
    acc = acc ? `${acc}/${part}` : part
    crumbs.append(document.createTextNode(' / '))
    const a = document.createElement('a')
    a.href = '#' + encodeURIComponent(acc)
    a.textContent = part
    crumbs.append(a)
  }
}

function row(entry, path) {
  const li = document.createElement('li')
  const full = path ? `${path}/${entry.name}` : entry.name
  const isDir = entry.type === 'directory'

  const ic = document.createElement('span')
  ic.className = 'icon'
  ic.append(icon(isDir ? FOLDER_ICON : FILE_ICON))

  const info = document.createElement('div')
  info.className = 'info'
  const name = document.createElement('div')
  name.className = 'name'
  const link = document.createElement('a')
  link.className = 'name-link'
  if (isDir) {
    link.href = '#' + encodeURIComponent(full)
  } else {
    link.href = urlFor(full)
    link.setAttribute('download', entry.name)
  }
  link.textContent = entry.name
  name.append(link)

  const meta = document.createElement('div')
  meta.className = 'meta'
  meta.textContent = isDir
    ? ['папка', humanDate(entry.mtime)].filter(Boolean).join(' · ')
    : [humanSize(entry.size), humanDate(entry.mtime)].filter(Boolean).join(' · ')

  info.append(name, meta)
  li.append(ic, info)

  if (!isDir) {
    const get = document.createElement('a')
    get.className = 'get'
    get.href = urlFor(full)
    get.setAttribute('download', entry.name)
    get.textContent = 'Скачать'
    li.append(get)
  }
  return li
}

function message(text, bad) {
  const p = document.createElement('p')
  p.className = 'msg' + (bad ? ' bad' : '')
  p.textContent = text
  out.replaceChildren(p)
}

async function load() {
  const path = currentPath()
  renderCrumbs(path)
  message('Загрузка…')

  let entries
  try {
    const res = await fetch(urlFor(path) + (path ? '/' : ''), { cache: 'no-store' })
    if (!res.ok) throw new Error(String(res.status))
    entries = await res.json()
  } catch {
    message('Не удалось прочитать папку.', true)
    return
  }
  if (!Array.isArray(entries)) {
    message('Не удалось прочитать папку.', true)
    return
  }

  // Hide dotfiles (.DS_Store and friends), folders first, then alphabetical.
  entries = entries.filter((e) => !e.name.startsWith('.'))
  entries.sort((a, b) => {
    const ad = a.type === 'directory'
    const bd = b.type === 'directory'
    if (ad !== bd) return ad ? -1 : 1
    return a.name.localeCompare(b.name, 'ru')
  })

  if (!entries.length) {
    message(path ? 'Папка пуста.' : 'Пока здесь пусто.')
    return
  }
  const ul = document.createElement('ul')
  for (const e of entries) ul.append(row(e, path))
  out.replaceChildren(ul)
}

window.addEventListener('hashchange', load)
load()
