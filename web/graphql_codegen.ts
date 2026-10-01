import { CodegenConfig } from '@graphql-codegen/cli';

const config: CodegenConfig = {
  schema: '../api/graphql/**/*.graphql',
  ignoreNoDocuments: true,
  generates: {
    './src/graphql/': {
      preset: 'client',
      documents: ['src/**/*.{ts,tsx}', '!src/ee/**/*.{ts,tsx}'],
      plugins: [],
      config: {
        useTypeImports: true
      }
    },
    './src/ee/graphql/': {
      preset: 'client',
      documents: ['src/ee/**/*.{ts,tsx}'],
      plugins: [],
      config: {
        useTypeImports: true
      }
    }
  }
};

export default config;
