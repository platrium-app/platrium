import { CodegenConfig } from '@graphql-codegen/cli';

const config: CodegenConfig = {
  schema: '../api/graphql/**/*.graphql',
  documents: ['src/**/*.{ts,tsx}'],
  generates: {
    './src/graphql/': {
      preset: 'client',
      plugins: [],
      config: {
        useTypeImports: true
      }
    }
  }
};

export default config;
