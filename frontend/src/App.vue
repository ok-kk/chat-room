<template>
  <div class="app">
    <router-view />
  </div>
</template>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body, #app { height: 100%; width: 100%; overflow: hidden; }
.app { height: 100%; background: #f0f2f5; }
::-webkit-scrollbar { width: 4px; }
::-webkit-scrollbar-thumb { background: #c0c4cc; border-radius: 2px; }
</style>