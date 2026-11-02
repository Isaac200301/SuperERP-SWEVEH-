import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'src/shared/api/schema.d.ts'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.strict],
    files: ['**/*.{ts,tsx}'],
    languageOptions: { ecmaVersion: 2022, globals: globals.browser },
    plugins: { 'react-hooks': reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'error',
      // Seguridad en el navegador (skill sweveh-frontend-react §10): nada de HTML crudo ni eval.
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-restricted-syntax': [
        'error',
        { selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']", message: 'Prohibido: riesgo de XSS.' },
        { selector: "MemberExpression[object.name='localStorage']", message: 'No se guardan datos de sesión en localStorage.' },
        { selector: "MemberExpression[object.name='sessionStorage']", message: 'No se guardan datos de sesión en sessionStorage.' },
      ],
    },
  },
);
