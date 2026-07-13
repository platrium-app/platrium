// @ts-nocheck
import * as __fd_glob_3 from "../content/docs/test-page/second-page.mdx?collection=docs"
import * as __fd_glob_2 from "../content/docs/test-page/index.mdx?collection=docs"
import * as __fd_glob_1 from "../content/docs/index.mdx?collection=docs"
import { default as __fd_glob_0 } from "../content/docs/test-page/meta.json?collection=docs"
import { server } from 'fumadocs-mdx/runtime/server';
import type * as Config from '../source.config';

const create = server<typeof Config, import("fumadocs-mdx/runtime/types").InternalTypeConfig & {
  DocData: {
  }
}>({"doc":{"passthroughs":["extractedReferences"]}});

export const docs = await create.docs("docs", "content/docs", {"test-page/meta.json": __fd_glob_0, }, {"index.mdx": __fd_glob_1, "test-page/index.mdx": __fd_glob_2, "test-page/second-page.mdx": __fd_glob_3, });