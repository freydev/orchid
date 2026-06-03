import svelte from 'rollup-plugin-svelte';
import sveltePreprocess from 'svelte-preprocess';
import resolve from '@rollup/plugin-node-resolve';
import commonjs from '@rollup/plugin-commonjs';
import typescript from '@rollup/plugin-typescript';
import css from 'rollup-plugin-css-only';
import terser from '@rollup/plugin-terser';

const prod = !process.env.ROLLUP_WATCH;
export default {
  input: 'src/main.ts',
  output: { file: 'dist/bundle.js', format: 'iife', name: 'app', sourcemap: !prod },
  plugins: [
    svelte({ preprocess: sveltePreprocess(), compilerOptions: { dev: !prod } }),
    css({ output: 'bundle.css' }),
    resolve({ browser: true, dedupe: ['svelte'], exportConditions: ['svelte', 'browser'] }),
    commonjs(),
    typescript({ tsconfig: './tsconfig.json', sourceMap: !prod }),
    prod && terser(),
  ],
  watch: { clearScreen: false },
};
