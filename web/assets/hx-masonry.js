/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

/**
 * <masonry-grid>
 *
 * A CSS-grid container whose children (.masonry-item) get absolutely positioned into a masonry layout: each card is
 * dropped into whichever column is currently shortest.
 *
 * The CSS grid is only used to *measure* things. We read the computed `grid-template-columns` to find out how many
 * columns the current breakpoint gives us and how wide each one is, then position cards ourselves. That way your
 * existing Tailwind classes (grid-cols-1 sm:grid-cols-2 xl:grid-cols-3, gap-*) stay the single source of truth for
 * column count and spacing.
 */
class MasonryGrid extends HTMLElement {
  // ---- private state -----------------------------------------------------

  /** Watches sizes. Observes the grid itself (width changes => column count may change) AND every card (height changes
   *  => cards below must move). */
  #ro

  /** Watches the grid's direct children being added/removed (pagination, morph swaps that insert new cards). */
  #mo

  /** Pending requestAnimationFrame id, or 0 if no layout is queued. Used to coalesce many triggers into a single layout
   *  per frame. */
  #raf = 0

  // Arrow functions so `this` is bound, and so the exact same function reference can be passed to removeEventListener
  // later.

  #onUpdate = () => this.schedule()
  #onPageShow = (e) => {
    // `persisted` is true when the page was restored from the back/forward cache. Layout may be stale (e.g. window
    // resized while away).
    if (e.persisted) this.schedule()
  }

  // ---- lifecycle ---------------------------------------------------------
  //
  // connectedCallback / disconnectedCallback fire whenever the element is inserted into / removed from the DOM,
  // regardless of HOW that happened (initial page load, htmx swap, idiomorph, history restore).

  connectedCallback() {
    // Any size change we observe just queues a layout.
    this.#ro = new ResizeObserver(() => this.schedule())
    // When children are added or removed, start observing the new ones and re-layout.
    this.#mo = new MutationObserver(() => {
      this.#observeItems()
      this.schedule()
    })
    this.#ro.observe(this) // container resized (viewport/breakpoint change)
    this.#mo.observe(this, { childList: true }) // direct children only, not subtree
    // Manual escape hatch: anything can do
    //   document.body.dispatchEvent(new Event('masonry:update'))
    // to force a relayout.
    document.body.addEventListener('masonry:update', this.#onUpdate)
    addEventListener('pageshow', this.#onPageShow)
    this.#observeItems()
    this.schedule()
  }

  disconnectedCallback() {
    // Clean up everything we attached outside this element. Observers and global listeners would otherwise leak every
    // time the grid is swapped out.
    this.#ro.disconnect()
    this.#mo.disconnect()
    cancelAnimationFrame(this.#raf)
    this.#raf = 0
    document.body.removeEventListener('masonry:update', this.#onUpdate)
    removeEventListener('pageshow', this.#onPageShow)
  }

  // ---- helpers -----------------------------------------------------------

  /**
   * The cards we lay out: direct children with class .masonry-item that aren't hidden. Selecting by class (instead of
   * "all children except X") means the pagination sentinel and stray <script> tags are never treated as cards. `:scope
   * >` restricts the match to direct children so nested elements that happen to have the class are ignored.
   */
  get items() {
    return [...this.querySelectorAll(':scope > .masonry-item')].filter(
      (el) => el.style.display !== 'none'
    )
  }

  /**
   * Start observing the size of every current card. This is what catches height changes that don't add/remove children:
   * an idiomorph swap changing a title's line count, web fonts finishing loading, images loading, etc. Calling
   * observe() on an element that's already observed is a harmless no-op, so we can call this after every mutation.
   */
  #observeItems() {
    for (const el of this.items) this.#ro.observe(el)
  }

  /**
   * Request a layout. Many triggers can fire in the same frame (a resize callback per card, a mutation, etc.); the #raf
   * guard collapses them into one layout pass just before the next paint.
   */
  schedule() {
    if (this.#raf) return
    this.#raf = requestAnimationFrame(() => {
      this.#raf = 0
      this.layout()
    })
  }

  // ---- the layout itself -------------------------------------------------

  layout() {
    const cs = getComputedStyle(this)
    // For a 3-column grid this is something like "374.158px 374.158px 374.158px". Splitting on whitespace gives one
    // entry per column track, so `cols` is the column count for the current breakpoint, and each entry is that column's
    // resolved width in px. (Assumes plain px tracks, which is what getComputedStyle returns for resolved grid-cols-N;
    // it would need more care for named lines or repeat() with subgrid.)
    const tracks = cs.gridTemplateColumns.trim().split(/\s+/)
    const cols = tracks.length
    // Gaps between columns and between rows. Tailwind's gap-2 / gap-4 / gap-8 set both. parseFloat strips the "px";
    // "normal" becomes NaN, so we fall back to 0.
    const colGap = parseFloat(cs.columnGap) || 0
    const rowGap = parseFloat(cs.rowGap) || 0
    const items = this.items

    if (cols === 1) {
      // ---- single column: undo everything and let the grid flow naturally. Clearing the inline styles lets the CSS
      // grid take over again, so we don't need absolute positioning on mobile at all.
      for (const el of items) {
        el.style.position = el.style.width = el.style.left = el.style.top = ''
        el.classList.add('masonry-positioned')
      }
      this.style.height = ''
    } else {
      // ---- multi column: true masonry.

      // Every column is the same width in this grid, so the first track's width is the card width.
      const w = parseFloat(tracks[0])
      // STEP 1: writes. Give every card its final width and take it out of flow. The width must be set BEFORE we
      // measure heights, because text wrapping (and therefore height) depends on the width.
      for (const el of items) {
        el.style.position = 'absolute'
        el.style.width = `${w}px`
      }
      // STEP 2: reads. Measure every card's height in one go. The first read forces one reflow; the rest are free. If
      // we instead interleaved "set style, read height, set style, read height" per card, each read would force its own
      // reflow (layout thrashing).
      const heights = items.map((el) => el.offsetHeight)
      // STEP 3: writes. Place each card. colY[c] = how far down column c is filled so far (px from the top of the
      // grid). All columns start empty.
      const colY = new Array(cols).fill(0)
      items.forEach((el, i) => {
        // Find the shortest column. Strict `<` means ties go to the leftmost column, so the first row fills left to
        // right.
        let c = 0
        for (let k = 1; k < cols; k++) if (colY[k] < colY[c]) c = k
        // x: skip c full columns, each (card width + gap) wide.
        // y: the current bottom of the chosen column.
        el.style.left = `${c * (w + colGap)}px`
        el.style.top = `${colY[c]}px`
        el.classList.add('masonry-positioned')
        // Grow that column by this card plus the gap before the next one.
        colY[c] += heights[i] + rowGap
      })
      // Absolutely positioned children take no space, so the container would collapse to 0 height. Set it explicitly to
      // the tallest column. We subtract rowGap once because colY includes a trailing gap after each column's last card.
      // `position: relative` makes the cards' left/top relative to the grid rather than to some ancestor.
      this.style.position = 'relative'
      this.style.height = `${Math.max(0, Math.max(...colY) - rowGap)}px`
    }
    this.dispatchEvent(new CustomEvent('gridLayoutComplete', { bubbles: true }))
  }
}
// Guard against "already defined" errors if this file is evaluated twice (e.g. re-included by a swapped fragment).
if (!customElements.get('masonry-grid')) customElements.define('masonry-grid', MasonryGrid)
