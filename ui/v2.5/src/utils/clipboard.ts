export async function copyText(text: string) {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(text);
      return;
    }
  } catch {
    // Native clipboard permissions and insecure HTTP can require the fallback.
  }
  const focused = document.activeElement;
  const input = document.createElement("textarea");
  input.value = text;
  input.setAttribute("aria-hidden", "true");
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  try {
    input.select();
    if (!document.execCommand("copy"))
      throw new Error(
        "Copy failed. Select the error text and copy it manually."
      );
  } finally {
    input.remove();
    if (focused instanceof HTMLElement) focused.focus();
  }
}
