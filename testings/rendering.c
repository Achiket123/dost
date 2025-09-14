#include <windows.h>

// Simple vertex format (position only)
typedef struct {
    float x, y, z;
} Vertex;

// Transformation matrix (4x4)
typedef struct {
    float m[4][4];
} Matrix4x4;

// Identity matrix
Matrix4x4 identityMatrix() {
     Matrix4x4 mat;
    mat.m[0][0] = 1.0f; mat.m[0][1] = 0.0f; mat.m[0][2] = 0.0f; mat.m[0][3] = 0.0f;
    mat.m[1][0] = 0.0f; mat.m[1][1] = 1.0f; mat.m[1][2] = 0.0f; mat.m[1][3] = 0.0f;
    mat.m[2][0] = 0.0f; mat.m[2][1] = 0.0f; mat.m[2][2] = 1.0f; mat.m[2][3] = 0.0f;
    mat.m[3][0] = 0.0f; mat.m[3][1] = 0.0f; mat.m[3][2] = 0.0f; mat.m[3][3] = 1.0f;
    return mat;mat;
}

// Translation matrix
Matrix4x4 translationMatrix(float x, float y, float z) {
    Matrix4x4 mat = identityMatrix();
    mat.m[3][0] = x;
    mat.m[3][1] = y;
    mat.m[3][2] = z;
    return mat;
}

// Scale matrix
Matrix4x4 scaleMatrix(float x, float y, float z) {
    Matrix4x4 mat = identityMatrix();
    mat.m[0][0] = x;
    mat.m[1][1] = y;
    mat.m[2][2] = z;
    return mat;
}

// Matrix multiplication
Vertex transformVertex(Vertex v, Matrix4x4 mat) {
    Vertex result;
    result.x = v.x * mat.m[0][0] + v.y * mat.m[1][0] + v.z * mat.m[2][0] + mat.m[3][0];
    result.y = v.x * mat.m[0][1] + v.y * mat.m[1][1] + v.z * mat.m[2][1] + mat.m[3][1];
    result.z = v.x * mat.m[0][2] + v.y * mat.m[1][2] + v.z * mat.m[2][2] + mat.m[3][2];
    return result;
}
