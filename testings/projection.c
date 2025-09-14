#include <math.h>

// Structure for 4x4 matrix
typedef struct {
    float m[4][4];
} Matrix4x4;

// Function to create a perspective projection matrix
Matrix4x4 perspectiveMatrix(
    float fieldOfView, // Field of view in radians
    float aspectRatio, // Aspect ratio (width/height)
    float nearPlane,   // Distance to the near clipping plane
    float farPlane     // Distance to the far clipping plane
) {
    float fovY = fieldOfView;
    float aspect = aspectRatio;
    float near = nearPlane;
    float far = farPlane;

    float yScale = 1.0f / tanf(fovY / 2.0f);
    float xScale = yScale / aspect;
    float frustumLength = far - near;

    Matrix4x4 m = {
        {{
            xScale, 0.0f,   0.0f,              0.0f,
            0.0f,   yScale, 0.0f,              0.0f,
            0.0f,   0.0f,   -((far+near) / frustumLength), -1.0f,
            0.0f,   0.0f,   -((2*near*far) / frustumLength), 0.0f
     Matrix4x4 m;
    m.m[0][0] = xScale; m.m[0][1] = 0.0f;   m.m[0][2] = 0.0f;              m.m[0][3] = 0.0f;
    m.m[1][0] = 0.0f;   m.m[1][1] = yScale; m.m[1][2] = 0.0f;              m.m[1][3] = 0.0f;
    m.m[2][0] = 0.0f;   m.m[2][1] = 0.0f;   m.m[2][2] = -((far+near) / frustumLength); m.m[2][3] = -1.0f;
    m.m[3][0] = 0.0f;   m.m[3][1] = 0.0f;   m.m[3][2] = -((2*near*far) / frustumLength); m.m[3][3] = 0.0f;
    return m;