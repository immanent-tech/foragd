/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

class MasonryGrid extends HTMLElement {
  #ro
  #mo
  #raf = 0
  #onUpdate = () => this.schedule()
  #onPageShow = (e) => {
    if (e.persisted) this.schedule()
  }

  connectedCallback() {
    this.#ro = new ResizeObserver(() => this.schedule())
    this.#mo = new MutationObserver(() => {
      this.#observeItems()
      this.schedule()
    })
    this.#ro.observe(this) // container width -> column count
    this.#mo.observe(this, { childList: true })
    document.body.addEventListener('masonry:update', this.#onUpdate)
    addEventListener('pageshow', this.#onPageShow)
    this.#observeItems()
    this.schedule()
  }

  disconnectedCallback() {
    this.#ro.disconnect()
    this.#mo.disconnect()
    cancelAnimationFrame(this.#raf)
    this.#raf = 0
    document.body.removeEventListener('masonry:update', this.#onUpdate)
    removeEventListener('pageshow', this.#onPageShow)
  }

  get items() {
    return [...this.querySelectorAll(':scope > .masonry-item')].filter(
      (el) => el.style.display !== 'none'
    )
  }

  // observe each card so content changes (morphs, fonts) trigger relayout
  #observeItems() {
    for (const el of this.items) this.#ro.observe(el)
  }

  schedule() {
    if (this.#raf) return
    this.#raf = requestAnimationFrame(() => {
      this.#raf = 0
      this.layout()
    })
  }

  layout() {
    const cs = getComputedStyle(this)
    const tracks = cs.gridTemplateColumns.trim().split(/\s+/)
    const cols = tracks.length
    const colGap = parseFloat(cs.columnGap) || 0
    const rowGap = parseFloat(cs.rowGap) || 0
    const items = this.items

    if (cols === 1) {
      for (const el of items) {
        el.style.position = el.style.width = el.style.left = el.style.top = ''
        el.classList.add('masonry-positioned')
      }
      this.style.height = ''
    } else {
      const w = parseFloat(tracks[0])
      for (const el of items) {
        // 1. writes
        el.style.position = 'absolute'
        el.style.width = `${w}px`
      }
      const heights = items.map((el) => el.offsetHeight) // 2. one reflow, all reads
      const colY = new Array(cols).fill(0)
      items.forEach((el, i) => {
        // 3. writes
        let c = 0
        for (let k = 1; k < cols; k++) if (colY[k] < colY[c]) c = k
        el.style.left = `${c * (w + colGap)}px`
        el.style.top = `${colY[c]}px`
        el.classList.add('masonry-positioned')
        colY[c] += heights[i] + rowGap
      })
      this.style.position = 'relative'
      this.style.height = `${Math.max(0, Math.max(...colY) - rowGap)}px`
    }
    this.dispatchEvent(new CustomEvent('gridLayoutComplete', { bubbles: true }))
  }
}
if (!customElements.get('masonry-grid')) customElements.define('masonry-grid', MasonryGrid)
