const updateEncoding = (encoding) => {
  const email = document.getElementById("email-value");
  const tag = document.getElementById("encoding-value");
  if (!email || !tag) return;

  const value = encoding === "hex" ? email.dataset.hexEmail : email.dataset.plainEmail;
  if (!value) return;

  email.textContent = value;
  tag.textContent = encoding.toUpperCase();
};

document.addEventListener("change", (event) => {
  if (event.target.name === "encoding") updateEncoding(event.target.value);
});

document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.target.id !== "alias-result") return;
  const selected = document.querySelector("input[name=encoding]:checked");
  if (selected) updateEncoding(selected.value);
});
