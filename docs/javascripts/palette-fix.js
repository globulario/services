/* Fix Material palette toggle for subdirectory serving (/docs/).
   Material's toggle handler uses __md_get/__md_set with a scope derived
   from new URL(".", location). This can produce inconsistent keys when
   served from a subdirectory. We take over the toggle behavior entirely. */
;(function () {
  var STORAGE_KEY = "globular-docs-palette"

  function getScheme() {
    return localStorage.getItem(STORAGE_KEY) || "default"
  }

  function setScheme(scheme) {
    localStorage.setItem(STORAGE_KEY, scheme)
  }

  function applyScheme(scheme) {
    document.body.setAttribute("data-md-color-scheme", scheme)
    document.body.setAttribute("data-md-color-primary", "custom")
    document.body.setAttribute("data-md-color-accent", "cyan")

    /* Check the correct radio so Material's CSS responds */
    var idx = scheme === "slate" ? 1 : 0
    var input = document.getElementById("__palette_" + idx)
    if (input) input.checked = true

    /* Show the label that toggles to the OTHER scheme */
    var labels = document.querySelectorAll("label[for^='__palette_']")
    for (var i = 0; i < labels.length; i++) {
      var forId = labels[i].getAttribute("for")
      if ((scheme === "default" && forId === "__palette_1") ||
          (scheme === "slate" && forId === "__palette_0")) {
        labels[i].removeAttribute("hidden")
      } else {
        labels[i].setAttribute("hidden", "")
      }
    }
  }

  function toggle() {
    var current = getScheme()
    var next = current === "default" ? "slate" : "default"
    setScheme(next)
    applyScheme(next)
  }

  function init() {
    /* Apply saved preference */
    var saved = getScheme()
    applyScheme(saved)

    /* Intercept clicks on both palette labels */
    var labels = document.querySelectorAll("label[for^='__palette_']")
    for (var i = 0; i < labels.length; i++) {
      labels[i].addEventListener("click", function (e) {
        e.preventDefault()
        e.stopPropagation()
        toggle()
      })
    }

    /* Also watch for Material re-hiding labels and fix them */
    var header = document.querySelector(".md-header__option")
    if (header) {
      new MutationObserver(function () {
        var allHidden = true
        var ls = header.querySelectorAll("label[for^='__palette_']")
        for (var k = 0; k < ls.length; k++) {
          if (!ls[k].hasAttribute("hidden")) { allHidden = false; break }
        }
        if (allHidden) applyScheme(getScheme())
      }).observe(header, { attributes: true, subtree: true, attributeFilter: ["hidden"] })
    }
  }

  /* Run on DOMContentLoaded to catch labels before Material's deferred init */
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { setTimeout(init, 50) })
  } else {
    setTimeout(init, 50)
  }
})()
