<template>
  <a
    class="lmspeed-badge-link"
    href="https://lmspeed.net/"
    target="_blank"
    rel="noopener noreferrer"
    aria-label="Featured on LMSpeed.net"
  >
    <img
      class="lmspeed-badge-image"
      src="https://lmspeed.net/api/provider/claim-badge/2039?claim=2039-1kHJJSboOUMX0Au9G01tAaBWozH20jbC"
      alt="Featured on LMSpeed.net"
      width="190"
      height="64"
      decoding="async"
    />
  </a>
</template>

<style scoped>
.lmspeed-badge-link {
  display: inline-flex;
  flex: 0 0 auto;
  max-width: 100%;
  border-radius: 10px;
}

.lmspeed-badge-link:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 4px;
}

.lmspeed-badge-image {
  display: block;
  width: 190px;
  max-width: 100%;
  height: auto;
}
</style>
