#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <windows.h>

// Vertex structure (x, y, z)
typedef struct {
    float x, y, z;
} Vertex;

// Face structure (indices of vertices)
typedef struct {
    int v1, v2, v3;
} Face;

// Model structure
typedef struct {
    Vertex* vertices;
    Face* faces;
    int numVertices;
    int numFaces;
} Model;

// Function to load .obj model from file
Model loadOBJ(const char* filename) {
    Model model = { 0 };
    FILE* file = fopen(filename, "r");
    if (file == NULL) {
        printf("Error opening file: %s\n", filename);
        return model;
    }

    // Count vertices and faces
    int verticesCount = 0;
    int facesCount = 0;
    char line[256];
    while (fgets(line, sizeof(line), file)) {
        if (strncmp(line, "v ", 2) == 0) {
            verticesCount++;
        } else if (strncmp(line, "f ", 2) == 0) {
            facesCount++;
        }
    }

    // Allocate memory for vertices and faces
    model.vertices = (Vertex*)malloc(verticesCount * sizeof(Vertex));
    model.faces = (Face*)malloc(facesCount * sizeof(Face));
    model.numVertices = verticesCount;
    model.numFaces = facesCount;

    // Reset file pointer to the beginning
    fseek(file, 0, SEEK_SET);

    // Read vertices and faces
    int vertexIndex = 0;
    int faceIndex = 0;
    while (fgets(line, sizeof(line), file)) {
        if (strncmp(line, "v ", 2) == 0) {
            sscanf(line, "v %f %f %f", &model.vertices[vertexIndex].x, &model.vertices[vertexIndex].y, &model.vertices[vertexIndex].z);
            vertexIndex++;
        } else if (strncmp(line, "f ", 2) == 0) {
            sscanf(line, "f %d %d %d", &model.faces[faceIndex].v1, &model.faces[faceIndex].v2, &model.faces[faceIndex].v3);
            // OBJ indices are 1-based, so subtract 1
            model.faces[faceIndex].v1--;
            model.faces[faceIndex].v2--;
            model.faces[faceIndex].v3--;
            faceIndex++;
        }
    }

    fclose(file);
    return model;
}

// Function to free the memory allocated for the model
void freeModel(Model* model) {
    if (model->vertices) free(model->vertices);
    if (model->faces) free(model->faces);
    model->numVertices = 0;
    model->numFaces = 0;
}

// dummy render function
void renderModel(HDC hdc, Model model) {
  // just print number of vertices and faces
  char buff[256];
  sprintf(buff, "Vertices: %d, Faces: %d", model.numVertices, model.numFaces);
  TextOutA(hdc, 10, 10, buff, strlen(buff));
}
