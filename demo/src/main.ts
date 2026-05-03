const grid = document.getElementById("grid")!;
const replayBtn = document.getElementById("replay-btn")!;

const DIGITS = [
  ...Array.from({ length: 10 }, (_, i) => ({
    char: String(i),
    code: 48 + i,
  })),
  ...Array.from({ length: 10 }, (_, i) => ({
    char: String.fromCodePoint(0xff10 + i),
    code: 0xff10 + i,
  })),
];

async function fetchSVG(code: number): Promise<string> {
  const resp = await fetch(`./svgsNumber/${code}.svg`);
  return resp.text();
}

async function loadAll() {
  grid.innerHTML = "";
  for (const d of DIGITS) {
    const raw = await fetchSVG(d.code);
    // Strip the XML comment header so we get just the <svg> element
    const svgMarkup = raw.replace(/<!--[\s\S]*?-->\s*/, "");

    const cell = document.createElement("div");
    cell.className = "cell";
    cell.innerHTML = `
      <div class="label">${d.char}</div>
      <div class="svg-container">${svgMarkup}</div>
    `;
    grid.appendChild(cell);
  }
}

replayBtn.addEventListener("click", () => loadAll());

loadAll();
