const domainInput = document.getElementById("domain");

if (domainInput) {
  const validDomain = (value) => {
    const normalized = value.trim().toLowerCase();
    const labels = normalized.split(".");
    return normalized.length <= 253 && labels.length > 0 && labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$|^[a-z0-9]$/.test(label));
  };

  const validateDomain = () => {
    domainInput.setCustomValidity(validDomain(domainInput.value) ? "" : "Enter a valid domain name.");
  };

  domainInput.addEventListener("input", validateDomain);
  domainInput.addEventListener("blur", validateDomain);
}
