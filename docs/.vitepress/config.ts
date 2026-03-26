import { createPhenotypeConfig } from "@phenotype/docs/config";

export default createPhenotypeConfig({
  title: "agentapi++",
  description: "Agent API server docs",
  base: process.env.GITHUB_ACTIONS ? "/agentapi-plusplus/" : "/",
  srcDir: ".",
  githubOrg: "KooshaPari",
  githubRepo: "agentapi-plusplus",
  nav: [
    { text: "Wiki", link: "/wiki/" },
    { text: "Development Guide", link: "/development-guide/" },
    { text: "Document Index", link: "/document-index/" },
    { text: "API", link: "/api/" },
    { text: "Roadmap", link: "/roadmap/" },
  ],
  sidebar: [
    {
      text: "Categories",
      items: [
        { text: "Wiki", link: "/wiki/" },
        { text: "Development Guide", link: "/development-guide/" },
        { text: "Document Index", link: "/document-index/" },
        { text: "API", link: "/api/" },
        { text: "Roadmap", link: "/roadmap/" },
      ],
    },
  ],
});
