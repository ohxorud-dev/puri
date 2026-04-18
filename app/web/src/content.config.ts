import { defineCollection, z } from "astro:content";
import { glob } from "astro/loaders";

const problemsCollection = defineCollection({
  loader: glob({ pattern: "**/*.md", base: "./src/content/problems" }),
  schema: z.object({
    title: z.string(),
    time_limit: z.string(),
    memory_limit: z.string(),
    tier: z.string().optional(),
    step: z.string().optional(),
    source: z.union([z.array(z.string()), z.string()]).optional(),
    examples: z
      .array(
        z.object({
          input: z.string(),
          output: z.string(),
        }),
      )
      .optional(),
  }),
});

export const collections = {
  problems: problemsCollection,
};
